package process

import (
	"sync"

	"github.com/serboox/supervisord/src/configs"
	log "github.com/sirupsen/logrus"
)

// Manager manage all the process in the supervisor
type Manager struct {
	processes map[string]*Process
	lock      sync.Mutex
}

// NewManager create a new Manager object
func NewManager() *Manager {
	return &Manager{
		processes: make(map[string]*Process),
	}
}

// CreateProcess create a program and add to this manager
func (pm *Manager) CreateProcess(config *configs.ProgramEntity) *Process {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	return pm.createProgram(config)
}

// StartAutoStartPrograms start all the program if its autostart is true
func (pm *Manager) StartAutoStartPrograms() {
	pm.ForEachProcess(func(proc *Process) {
		if proc.isAutoStart() {
			proc.Start(false)
		}
	})
}

func (pm *Manager) createProgram(config *configs.ProgramEntity) *Process {
	procName := config.GetProgramName()

	proc, ok := pm.processes[procName]

	if !ok {
		proc = NewProcess(config)
		pm.processes[procName] = proc
	}

	log.Info("create process:", procName)

	return proc
}

// Add add the process to this process manager
func (pm *Manager) Add(name string, proc *Process) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.processes[name] = proc

	log.Info("add process:", name)
}

// Remove remove the process from the manager
func (pm *Manager) Remove(name string) *Process {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	proc, exist := pm.processes[name]

	if !exist {
		return nil
	}

	delete(pm.processes, name)

	log.Info("remove process:", name)

	return proc
}

// ForEachProcess process each process in sync mode
func (pm *Manager) ForEachProcess(procFunc func(p *Process)) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	procs := pm.getAllProcess()
	for _, proc := range procs {
		procFunc(proc)
	}
}

func (pm *Manager) getAllProcess() (procSlice []*Process) {
	procSlice = make([]*Process, 0)
	for _, proc := range pm.processes {
		procSlice = append(procSlice, proc)
	}

	return
}

// StopAllProcesses stop all the processes managed by this manager
func (pm *Manager) StopAllProcesses() {
	var wg sync.WaitGroup

	pm.ForEachProcess(func(proc *Process) {
		wg.Add(1)

		go func(wg *sync.WaitGroup) {
			defer wg.Done()

			proc.Stop(true)
		}(&wg)
	})

	wg.Wait()
}
