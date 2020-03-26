package process

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/serboox/supervisord/src/configs"
	"github.com/serboox/supervisord/src/signals"
	"github.com/serboox/supervisord/src/utils"
	log "github.com/sirupsen/logrus"
)

// State the state of process
type State int

const (
	// Stopped the stopped state
	Stopped State = iota

	// Starting the starting state
	Starting = 10

	// Running the running state
	Running = 20

	// Backoff the backoff state
	Backoff = 30

	// Stopping the stopping state
	Stopping = 40

	// Exited the Exited state
	Exited = 100

	// Fatal the Fatal state
	Fatal = 200

	// Unknown the unknown state
	Unknown = 1000
)

// String convert State to human readable string
func (p State) String() string {
	switch p {
	case Stopped:
		return "Stopped"
	case Starting:
		return "Starting"
	case Running:
		return "Running"
	case Backoff:
		return "Backoff"
	case Stopping:
		return "Stopping"
	case Exited:
		return "Exited"
	case Fatal:
		return "Fatal"
	default:
		return "Unknown"
	}
}

// Process the program process management data
type Process struct {
	config    *configs.ProgramEntity
	cmd       *exec.Cmd
	startTime time.Time
	stopTime  time.Time
	state     State
	//true if process is starting
	inStart bool
	//true if the process is stopped by user
	stopByUser bool
	retryTimes *int32
	stdin      io.WriteCloser
	StdoutLog  io.Writer
	StderrLog  io.Writer

	lock sync.RWMutex
}

// NewProcess create a new Process
func NewProcess(config *configs.ProgramEntity) *Process {
	proc := &Process{
		config:     config,
		cmd:        nil,
		startTime:  time.Unix(0, 0),
		stopTime:   time.Unix(0, 0),
		state:      Stopped,
		inStart:    false,
		stopByUser: false,
		retryTimes: new(int32)}
	proc.config = config
	proc.cmd = nil
	proc.StdoutLog = os.Stdout
	proc.StderrLog = os.Stderr

	return proc
}

// GetName get the name of program
func (p *Process) GetName() string {
	return p.config.GetProgramName()
}

func (p *Process) getStartSeconds() int64 {
	return p.config.GetStartSeconds()
}

func (p *Process) getRestartPause() uint {
	return p.config.GetRestartPause()
}

func (p *Process) getStartRetries() int32 {
	return int32(p.config.GetStartRetries())
}

func (p *Process) isAutoStart() bool {
	return p.config.IsAutoStart()
}

// Start start the process
// Args:
//  wait - true, wait the program started or failed
func (p *Process) Start(wait bool) {
	log.WithFields(log.Fields{"program": p.GetName()}).Info("try to start program")
	p.lock.Lock()
	if p.inStart {
		log.WithFields(log.Fields{"program": p.GetName()}).Info("Don't start program again, program is already started")
		p.lock.Unlock()

		return
	}

	p.inStart = true
	p.stopByUser = false
	p.lock.Unlock()

	var runCond *sync.Cond

	finished := false

	if wait {
		runCond = sync.NewCond(&sync.Mutex{})
		runCond.L.Lock()
	}

	go func() {
		for {
			if wait {
				runCond.L.Lock()
			}

			p.run(func() {
				finished = true
				if wait {
					runCond.L.Unlock()
					runCond.Signal()
				}
			})
			//avoid print too many logs if fail to start program too quickly
			if time.Now().Unix()-p.startTime.Unix() < 2 {
				time.Sleep(5 * time.Second)
			}

			if p.stopByUser {
				log.WithFields(log.Fields{"program": p.GetName()}).Info("Stopped by user, don't start it again")
				break
			} else {
				log.WithFields(log.Fields{"program": p.GetName()}).Info("Stopped by enemy, try start it again")
			}
		}
		p.lock.Lock()
		p.inStart = false
		p.lock.Unlock()
	}()

	if wait && !finished {
		runCond.Wait()
		runCond.L.Unlock()
	}
}

func (p *Process) run(finishCb func()) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// check if the program is in running state
	if p.isRunning() {
		log.WithFields(log.Fields{"program": p.GetName()}).Info("Don't start program because it is running")
		finishCb()

		return
	}

	p.startTime = time.Now()
	atomic.StoreInt32(p.retryTimes, 0)
	startSecs := p.getStartSeconds()
	restartPause := p.getRestartPause()

	var once sync.Once

	// finishCb can be only called one time
	finishCbWrapper := func() {
		once.Do(finishCb)
	}
	//process is not expired and not stoped by user
	for !p.stopByUser {
		if restartPause > 0 && atomic.LoadInt32(p.retryTimes) != 0 {
			//pause
			p.lock.Unlock()
			log.WithFields(log.Fields{"program": p.GetName()}).Info("don't restart the program, start it after ", restartPause, " seconds")
			time.Sleep(time.Duration(restartPause) * time.Second)
			p.lock.Lock()
		}

		endTime := time.Now().Add(time.Duration(startSecs) * time.Second)

		p.changeStateTo(Starting)
		atomic.AddInt32(p.retryTimes, 1)

		err := p.createProgramCommand()
		if err != nil {
			p.failToStartProgram("fail to create program", finishCbWrapper)
			break
		}

		err = p.cmd.Start()

		if err != nil {
			if atomic.LoadInt32(p.retryTimes) >= p.getStartRetries() {
				p.failToStartProgram(fmt.Sprintf("fail to start program with error:%v", err), finishCbWrapper)
				break
			} else {
				log.WithFields(log.Fields{"program": p.GetName()}).Info("fail to start program with error:", err)
				p.changeStateTo(Backoff)
				continue
			}
		}

		monitorExited := int32(0)
		programExited := int32(0)
		//Set startsec to 0 to indicate that the program needn't stay
		//running for any particular amount of time.
		if startSecs <= 0 {
			log.WithFields(log.Fields{"program": p.GetName()}).Info("success to start program")
			p.changeStateTo(Running)

			go finishCbWrapper()
		} else {
			go func() {
				p.monitorProgramIsRunning(endTime, &monitorExited, &programExited)
				finishCbWrapper()
			}()
		}
		p.lock.Unlock()

		healthCtx, cancelHealthCheck := context.WithCancel(context.Background())
		go p.healthChecker(healthCtx)

		log.WithFields(log.Fields{"program": p.GetName()}).Debug("wait program exit")
		p.waitForExit()
		cancelHealthCheck()

		atomic.StoreInt32(&programExited, 1)
		// wait for monitor thread exit
		for atomic.LoadInt32(&monitorExited) == 0 {
			time.Sleep(time.Duration(10) * time.Millisecond)
		}

		p.lock.Lock()

		// if the program still in running after startSecs
		if p.state == Running {
			p.changeStateTo(Exited)
			log.WithFields(log.Fields{"program": p.GetName()}).Info("program exited")

			break
		} else {
			p.changeStateTo(Backoff)
		}

		// The number of serial failure attempts that supervisord will allow when attempting to
		// start the program before giving up and putting the process into an Fatal state
		// first start time is not the retry time
		if atomic.LoadInt32(p.retryTimes) >= p.getStartRetries() {
			p.failToStartProgram(fmt.Sprintf("fail to start program because retry times is greater than %d", p.getStartRetries()), finishCbWrapper)
			break
		}
	}
}

// Checking the health of the process and restarting in case of errors
func (p *Process) healthChecker(ctx context.Context) {
	var (
		attempts uint
		check    = true
	)
LOOP:
	for check {
		delay := time.Duration(p.config.GetHealthAttemptsDelayMilliseconds())
		if p.state != Running {
			time.Sleep(delay * time.Millisecond)
			continue
		}

		select {
		case <-time.After(delay * time.Millisecond):
			if attempts >= p.config.GetHealthAttempts() {
				ctx.Done()
				check = false
				_ = p.Signal(syscall.SIGKILL, true)
				break LOOP
			}

			healthURL := url.URL{
				Scheme: p.config.GetHealthHostScheme(),
				Host: fmt.Sprintf(
					"%s:%d",
					p.config.GetHealthHost(),
					p.config.GetHealthPort(),
				),
				Path: p.config.GetHealthPath(),
			}
			log.WithFields(log.Fields{"program": p.GetName()}).Debug("Response url: ", healthURL.String())
			resp, err := http.Get(healthURL.String())
			if err != nil {
				log.Error(err)
				attempts++
				continue
			}
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				log.WithFields(log.Fields{"program": p.GetName()}).Debug("Response status: ", resp.Status)
				attempts = 0
				continue
			}
			attempts++
			log.WithFields(log.Fields{"program": p.GetName(), "attempt": attempts}).
				Warn("Response status: ", resp.Status)
		case <-ctx.Done():
			check = false
			break LOOP
		}
	}
}

// monitor if the program is in running before endTime
//
func (p *Process) monitorProgramIsRunning(endTime time.Time, monitorExited *int32, programExited *int32) {
	// if time is not expired
	for time.Now().Before(endTime) && atomic.LoadInt32(programExited) == 0 {
		time.Sleep(time.Duration(100) * time.Millisecond)
	}
	atomic.StoreInt32(monitorExited, 1)

	p.lock.Lock()
	defer p.lock.Unlock()
	// if the program does not exit
	if atomic.LoadInt32(programExited) == 0 && p.state == Starting {
		log.WithFields(log.Fields{"program": p.GetName()}).Info("success to start program")
		p.changeStateTo(Running)
	}
}

// check if the process is running or not
func (p *Process) isRunning() bool {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Signal(syscall.Signal(0)) == nil
	}

	return false
}

// create Command object for the program
func (p *Process) createProgramCommand() error {
	args, err := utils.ParseCommand(p.config.GetCommand())

	if err != nil {
		return err
	}

	p.cmd = exec.Command(args[0])

	if len(args) > 1 {
		p.cmd.Args = args
	}

	p.cmd.SysProcAttr = &syscall.SysProcAttr{}
	if p.setUser() != nil {
		log.WithFields(log.Fields{"user": p.config.GetUser()}).Error("fail to run as user")
		return fmt.Errorf("fail to set user")
	}

	setDeathsig(p.cmd.SysProcAttr)

	p.setEnv()
	p.setDir()
	p.setLog()

	p.stdin, _ = p.cmd.StdinPipe()

	return nil
}

func (p *Process) setEnv() {
	envSlice := p.config.GetEnv()
	if len(envSlice) != 0 {
		p.cmd.Env = append(os.Environ(), envSlice...)
	} else {
		p.cmd.Env = os.Environ()
	}
}

// Signal send signal to the process
//
// Args:
//   sig - the signal to the process
//   sigChildren - true: send the signal to the process and its children proess
//
func (p *Process) Signal(sig os.Signal, sigChildren bool) error {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.sendSignal(sig, sigChildren)
}

// send signal to the process
//
// Args:
//    sig - the signal to be sent
//    sigChildren - true if the signal also need to be sent to children process
//
func (p *Process) sendSignal(sig os.Signal, sigChildren bool) error {
	if p.cmd != nil && p.cmd.Process != nil {
		err := signals.Kill(p.cmd.Process, sig, sigChildren)
		return err
	}

	return fmt.Errorf("process is not started")
}

func (p *Process) setDir() {
	if dir := p.config.GetDirectory(); dir != "" {
		p.cmd.Dir = dir
	}
}

func (p *Process) setUser() error {
	userName := p.config.GetUser()
	if len(userName) == 0 {
		return nil
	}

	//check if group is provided
	pos := strings.Index(userName, ":")
	groupName := ""

	if pos != -1 {
		groupName = userName[pos+1:]
		userName = userName[0:pos]
	}

	u, err := user.Lookup(userName)
	if err != nil {
		return err
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return err
	}

	gid, err := strconv.ParseUint(u.Gid, 10, 32)

	if err != nil && groupName == "" {
		return err
	}

	if groupName != "" {
		g, err := user.LookupGroup(groupName)
		if err != nil {
			return err
		}

		gid, err = strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			return err
		}
	}

	setUserID(p.cmd.SysProcAttr, uint32(uid), uint32(gid))

	return nil
}

func (p *Process) setLog() {
	p.cmd.Stdout = p.StdoutLog
	p.cmd.Stderr = p.StderrLog
}

func setUserID(procAttr *syscall.SysProcAttr, uid uint32, gid uint32) {
	procAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid, NoSetGroups: true}
}

// Stop send signal to process to stop it
func (p *Process) Stop(wait bool) {
	p.lock.Lock()
	p.stopByUser = true
	isRunning := p.isRunning()
	p.lock.Unlock()

	if !isRunning {
		log.WithFields(log.Fields{"program": p.GetName()}).Info("program is not running")
		return
	}

	log.WithFields(log.Fields{"program": p.GetName()}).Info("stop the program")

	sigs := strings.Fields(p.config.GetStopSignal())
	waitsecs := time.Duration(p.config.GetStopWaitSecs()) * time.Second

	go func() {
		stopped := false
		for i := 0; i < len(sigs) && !stopped; i++ {
			// send signal to process
			sig, err := signals.ToSignal(sigs[i])
			if err != nil {
				continue
			}

			log.WithFields(log.Fields{"program": p.GetName(), "signal": sigs[i]}).Info("send stop signal to program")

			_ = p.Signal(sig, true)
			endTime := time.Now().Add(waitsecs)
			//wait at most "stopwaitsecs" seconds for one signal
			for endTime.After(time.Now()) {
				//if it already exits
				if p.state != Starting && p.state != Running && p.state != Stopping {
					stopped = true
					break
				}

				time.Sleep(1 * time.Second)
			}
		}

		if !stopped {
			log.WithFields(log.Fields{"program": p.GetName()}).Info("force to kill the program")

			_ = p.Signal(syscall.SIGKILL, true)
		}
	}()

	if wait {
		for {
			// if the program exits
			p.lock.RLock()
			if p.state != Starting && p.state != Running && p.state != Stopping {
				p.lock.RUnlock()
				break
			}
			p.lock.RUnlock()
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *Process) changeStateTo(procState State) {
	p.state = procState
}

// wait for the started program exit
func (p *Process) waitForExit() {
	err := p.cmd.Wait()

	switch {
	case err != nil:
		log.WithFields(log.Fields{"program": p.GetName()}).Info("fail to wait for program exit")
	case p.cmd.ProcessState != nil:
		log.WithFields(log.Fields{"program": p.GetName()}).Infof("program stopped with status:%v", p.cmd.ProcessState)
	default:
		log.WithFields(log.Fields{"program": p.GetName()}).Info("program stopped")
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	p.stopTime = time.Now()
}

// fail to start the program
func (p *Process) failToStartProgram(reason string, finishCb func()) {
	log.WithFields(log.Fields{"program": p.GetName()}).Errorf(reason)
	p.changeStateTo(Fatal)
	finishCb()
}
