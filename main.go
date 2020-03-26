package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/serboox/supervisord/src/supervisor"
	"github.com/serboox/supervisord/src/version"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func main() {
	log.Info("Repository: ", version.Repository)
	log.Info("Release: ", version.Release)
	log.Info("Commit: ", version.Commit)
	log.Info("BuildTime: ", version.BuildTime)
	log.Info("Supervisor PID: ", os.Getpid())

	runServer()
}

func runServer() {
	for {
		s := supervisor.NewSupervisor()
		initSignals(s)
		s.CreatePrograms()
		s.StartAutoStartPrograms()
		s.WaitForExit()
	}
}

func initSignals(s *supervisor.Supervisor) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs

		log.WithFields(log.Fields{"signal": sig}).Info("receive a signal to stop all process & exit")

		s.ProcessManager.StopAllProcesses()
		os.Exit(-1)
	}()
}
