package supervisor

import (
	"time"

	"github.com/serboox/supervisord/src/configs"
	"github.com/serboox/supervisord/src/process"
)

// Supervisor manage all the processes defined in the supervisor configuration file.
type Supervisor struct {
	Config         *configs.Config
	ProcessManager *process.Manager
	Restarting     bool
}

// NewSupervisor create a Supervisor object with supervisor configuration file
func NewSupervisor() *Supervisor {
	return &Supervisor{
		Config:         configs.NewConfig(),
		ProcessManager: process.NewManager(),
		Restarting:     false,
	}
}

// WaitForExit wait the superior to exit
func (s *Supervisor) WaitForExit() {
	for {
		if s.IsRestarting() {
			s.ProcessManager.StopAllProcesses()
			break
		}

		time.Sleep(10 * time.Second)
	}
}

// CreatePrograms create all program process
func (s *Supervisor) CreatePrograms() {
	for _, program := range s.Config.GetPrograms() {
		s.ProcessManager.CreateProcess(program)
	}
}

// StartAutoStartPrograms start all autostart programs
func (s *Supervisor) StartAutoStartPrograms() {
	s.ProcessManager.StartAutoStartPrograms()
}

// IsRestarting check if supervisor is in restarting state
func (s *Supervisor) IsRestarting() bool {
	return s.Restarting
}
