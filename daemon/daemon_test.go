package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/kardianos/service"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
)

type fakeService struct {
	service.Service
	installed, running bool
	calls              []string
}

func (f *fakeService) Status() (service.Status, error) {
	f.calls = append(f.calls, "status")
	switch {
	case !f.installed:
		return service.StatusUnknown, service.ErrNotInstalled
	case f.running:
		return service.StatusRunning, nil
	}
	return service.StatusStopped, nil
}

func (f *fakeService) Install() error {
	f.calls = append(f.calls, "install")
	f.installed = true
	return nil
}

func (f *fakeService) Uninstall() error {
	f.calls = append(f.calls, "uninstall")
	f.installed = false
	return nil
}

func (f *fakeService) Start() error {
	f.calls = append(f.calls, "start")
	f.running = true
	return nil
}

func (f *fakeService) Stop() error {
	f.calls = append(f.calls, "stop")
	f.running = false
	return nil
}

type fakeSystem struct {
	configs []*service.Config
}

func (*fakeSystem) String() string    { return "fake" }
func (*fakeSystem) Detect() bool      { return true }
func (*fakeSystem) Interactive() bool { return true }

func (s *fakeSystem) New(_ service.Interface, c *service.Config) (service.Service, error) {
	s.configs = append(s.configs, c)
	return &fakeService{}, nil
}

func useFake(t *testing.T, installed, running bool) *fakeService {
	fake := &fakeService{installed: installed, running: running}
	previous, previousLogs := globalDaemon, logDirectory
	globalDaemon, logDirectory = func() (service.Service, error) { return fake, nil }, filepath.Join(t.TempDir(), "logs")
	t.Cleanup(func() { globalDaemon, logDirectory = previous, previousLogs })
	return fake
}

func useSystems(t *testing.T, systems ...service.System) {
	previous, previousDaemon, previousLogs := service.AvailableSystems(), globalDaemon, logDirectory
	if previousLogs != "" {
		t.Fatal("log directory computed before the daemon was first used")
	}
	service.ChooseSystem(systems...)
	globalDaemon = sync.OnceValues(newDaemon)
	t.Cleanup(func() {
		service.ChooseSystem(previous...)
		globalDaemon, logDirectory = previousDaemon, previousLogs
	})
}

func TestInstallCreatesLogDirectory(t *testing.T) {
	useFake(t, false, false)
	Install()
	if _, err := os.Stat(logDirectory); err != nil {
		t.Fatal(err)
	}
}

func TestIsRunning(t *testing.T) {
	for _, tt := range []struct {
		name               string
		installed, running bool
		want               bool
	}{
		{"missing", false, false, false},
		{"stopped", true, false, true},
		{"running", true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := useFake(t, tt.installed, tt.running)
			if got := IsRunning(); got != tt.want || !reflect.DeepEqual(fake.calls, []string{"status"}) {
				t.Fatalf("IsRunning() = %v, calls = %q, want %v", got, fake.calls, tt.want)
			}
		})
	}
}

func TestManagerCalls(t *testing.T) {
	for _, tt := range []struct {
		name               string
		action             func()
		installed, running bool
		want               []string
	}{
		{"install missing", Install, false, false, []string{"status", "install"}},
		{"install stopped", Install, true, false, []string{"status"}},
		{"install running", Install, true, true, []string{"status"}},
		{"start stopped", Start, true, false, []string{"status", "start"}},
		{"start running", Start, true, true, []string{"status"}},
		{"stop missing", Stop, false, false, []string{"status"}},
		{"stop stopped", Stop, true, false, []string{"status"}},
		{"stop running", Stop, true, true, []string{"status", "stop"}},
		{"remove missing", Remove, false, false, []string{"status"}},
		{"remove stopped", Remove, true, false, []string{"status", "uninstall"}},
		{"remove running", Remove, true, true, []string{"status", "uninstall"}},
		{"status missing", Status, false, false, []string{"status"}},
		{"status running", Status, true, true, []string{"status"}},
		{"run start missing", func() { Run("start") }, false, false, []string{"status", "install", "status", "start"}},
		{"run remove running", func() { Run("remove") }, true, true, []string{"status", "stop", "status", "uninstall"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := useFake(t, tt.installed, tt.running)
			tt.action()
			if !reflect.DeepEqual(fake.calls, tt.want) {
				t.Fatalf("calls = %q, want %q", fake.calls, tt.want)
			}
		})
	}
}

func TestNewDaemonOnDemand(t *testing.T) {
	system := &fakeSystem{}
	useSystems(t, system)
	dir, err := config.DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	logs := filepath.Join(dir, "logs")
	want := &service.Config{
		Name:        jumpway.AppName,
		DisplayName: jumpway.AppName,
		Description: jumpway.AppDescription,
		Option: service.KeyValue{
			"UserService":  runtime.GOOS == "darwin",
			"RunAtLoad":    true,
			"KeepAlive":    false,
			"LogDirectory": logs,
		},
	}
	IsRunning()
	IsRunning()
	if len(system.configs) != 1 {
		t.Fatalf("created %d services for two queries", len(system.configs))
	}
	if !reflect.DeepEqual(system.configs[0], want) || logDirectory != logs {
		t.Fatalf("config = %+v, log directory = %q", *system.configs[0], logDirectory)
	}
}

func TestNoServiceSystem(t *testing.T) {
	useSystems(t)
	Stop()
	Remove()
	if IsRunning() {
		t.Fatal("IsRunning() without a service manager")
	}
	if _, err := globalDaemon(); !errors.Is(err, service.ErrNoServiceSystemDetected) {
		t.Fatalf("initialization error = %v", err)
	}
}
