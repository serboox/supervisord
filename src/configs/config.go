package configs

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

// ProgramEntity the options for supervise app running over command-line
type ProgramEntity struct {
	ProgramName  string `yaml:"program_name"`
	Command      string `yaml:"command"`
	Directory    string `yaml:"directory"`
	AutoStart    bool   `yaml:"autostart"`
	AutoRestart  bool   `yaml:"autorestart"`
	StartSecs    uint   `yaml:"startsecs"`
	RestartPause uint   `yaml:"restartpause"`
	StartRetries uint   `yaml:"startretries"`
	StopSignal   string `yaml:"stopsignal"`
	StopWaitSecs uint   `yaml:"stopwaitsecs"`
	User         string `yaml:"user"`
	Environment  []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	} `yaml:"environment"`
	Health struct {
		Scheme                    string `yaml:"scheme"`
		Host                      string `yaml:"host"`
		Port                      uint   `yaml:"port"`
		Path                      string `yaml:"path"`
		Attempts                  uint   `yaml:"attempts"`
		AttemptsDelayMilliseconds uint   `yaml:"attempts_delay_milliseconds"`
	} `yaml:"health"`
}

// GetProgramName get the program name
func (pe *ProgramEntity) GetProgramName() string {
	return pe.ProgramName
}

// GetCommand get the command
func (pe *ProgramEntity) GetCommand() string {
	return pe.Command
}

// GetDirectory get the directory
func (pe *ProgramEntity) GetDirectory() string {
	return pe.Directory
}

// GetUser get the user
func (pe *ProgramEntity) GetUser() string {
	return pe.User
}

// GetStartSeconds get start seconds
func (pe *ProgramEntity) GetStartSeconds() int64 {
	if pe.StartSecs == 0 {
		return 1
	}

	return int64(pe.StartSecs)
}

// GetRestartPause get restart pause
func (pe *ProgramEntity) GetRestartPause() uint {
	return pe.RestartPause
}

// GetStartRetries get start retries
func (pe *ProgramEntity) GetStartRetries() uint {
	return pe.StartRetries
}

// IsAutoStart is auto start
func (pe *ProgramEntity) IsAutoStart() bool {
	return pe.AutoStart
}

// GetStopSignal get stop signal
func (pe *ProgramEntity) GetStopSignal() string {
	return pe.StopSignal
}

// GetStopWaitSecs get stop wait secs
func (pe *ProgramEntity) GetStopWaitSecs() uint {
	if pe.StopWaitSecs <= 10 {
		return 10
	}

	return pe.StopWaitSecs
}

// GetGroupName get the group name
func (pe *ProgramEntity) GetGroupName() string {
	return ""
}

// GetGetExitCodes get the exit codes
func (pe *ProgramEntity) GetGetExitCodes() string {
	return "0,2"
}

// GetEnv build environment string array
// Each entry is of the form "key=value".
func (pe *ProgramEntity) GetEnv() (res []string) {
	countVars := len(pe.Environment)
	if countVars == 0 {
		return
	}

	res = make([]string, countVars)

	for i, envVar := range pe.Environment {
		res[i] = fmt.Sprintf("%s=%s", envVar.Name, envVar.Value)
	}

	return
}

// GetHealthHostScheme get the health host scheme
func (pe *ProgramEntity) GetHealthHostScheme() string {
	if pe.Health.Scheme != "http" &&
		pe.Health.Scheme != "https" {
		return "http"
	}

	return pe.Health.Scheme
}

// GetHealthHost get the health host
func (pe *ProgramEntity) GetHealthHost() string {
	return pe.Health.Host
}

// GetHealthPort get the health port
func (pe *ProgramEntity) GetHealthPort() uint {
	if pe.Health.Port == 0 {
		return 80
	}

	return pe.Health.Port
}

// GetHealthPath get the health path
func (pe *ProgramEntity) GetHealthPath() string {
	return pe.Health.Path
}

// GetHealthAttempts get the health attempts
func (pe *ProgramEntity) GetHealthAttempts() uint {
	return pe.Health.Attempts
}

// GetHealthAttemptsDelayMilliseconds get the health attempts in milliseconds
func (pe *ProgramEntity) GetHealthAttemptsDelayMilliseconds() int64 {
	if pe.Health.AttemptsDelayMilliseconds <= 50 {
		return 50
	}

	return int64(pe.Health.AttemptsDelayMilliseconds)
}

// Config the application options
type Config struct {
	Programs []*ProgramEntity `yaml:"programs"`
}

// NewConfig return new config exemplar
func NewConfig() *Config {
	config := new(Config)
	config.ParseFromFile()

	return config
}

// GetProgramNames return all program names
func (c *Config) GetProgramNames() (names []string) {
	names = make([]string, len(c.Programs))
	for i, program := range c.Programs {
		names[i] = program.ProgramName
	}

	return
}

// GetPrograms return all program
func (c *Config) GetPrograms() []*ProgramEntity {
	return c.Programs
}

// ParseFromFile parsing config file
func (c *Config) ParseFromFile() {
	path, err := c.findConfig()
	if err != nil {
		log.Fatal(err)
	}

	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(configBytes, c)
	if err != nil {
		log.Fatal(err)
	}
}

func (c *Config) findConfig() (string, error) {
	configPatches := []string{
		"./supervisord-conf.yaml",
		"./etc/supervisord-conf.yaml",
		"/etc/supervisord-conf.yaml",
		"/etc/supervisor/supervisord-conf.yaml",
	}

	for _, patch := range configPatches {
		if _, err := os.Stat(patch); err == nil {
			absPatch, err := filepath.Abs(patch)
			if err == nil {
				return absPatch, nil
			}

			return patch, nil
		}
	}

	return "", fmt.Errorf("fail to find supervisord-conf.yaml")
}
