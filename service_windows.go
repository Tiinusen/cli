//go:build windows

package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// Daemonize makes the application work as a linux/windows service
func (b *Builder) Daemonize(serviceName string, serviceDescription string) *Builder {
	if b.daemoize {
		return b
	}
	inService, err := svc.IsWindowsService()
	if err != nil {
		panic(err)
	}
	if inService {
		exepath, err := exePath()
		if err != nil {
			panic(err)
		}
		elog, err = eventlog.Open(serviceName)
		if err != nil {
			panic(err)
		}
		os.Chdir(filepath.Dir(exepath)) // To ensure current working directory is the same folder as the binary
		logger := &ExecuteLogger{b: b}
		log.SetFlags(0)
		log.SetOutput(logger)
		go func() {
			defer elog.Close()
			if err := svc.Run(serviceName, logger); err != nil {
				log.Fatal("background service failed")
			}
		}()
		return b
	}
	b.daemoize = true
	b.Command("install", "installs windows service", func(runner *Runner, args Args, flags Flags) error {
		controlService(serviceName, svc.Stop, svc.Stopped) // In case service is started
		exepath, err := exePath()
		if err != nil {
			return err
		}
		m, err := mgr.Connect()
		if err != nil {
			return err
		}
		defer m.Disconnect()
		s, err := m.OpenService(serviceName)
		if err == nil {
			s.Close()
			return fmt.Errorf("service %s already exists", serviceName)
		}
		s, err = m.CreateService(serviceName, exepath, mgr.Config{DisplayName: serviceName, Description: serviceDescription, StartType: mgr.StartAutomatic, DelayedAutoStart: true}) // Supports args
		if err != nil {
			return err
		}
		defer s.Close()
		err = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
		if err != nil {
			s.Delete()
			return fmt.Errorf("SetupEventLogSource() failed: %s", err)
		}
		if err := startService(serviceName); err != nil {
			return err
		}
		log.Println("service was successfully installed and started")
		os.Exit(0)
		return nil
	})
	b.Command("remove", "uninstalls windows service", func(runner *Runner, args Args, flags Flags) error {
		controlService(serviceName, svc.Stop, svc.Stopped) // In case service is started
		m, err := mgr.Connect()
		if err != nil {
			return err
		}
		defer m.Disconnect()
		s, err := m.OpenService(serviceName)
		if err != nil {
			return fmt.Errorf("service %s is not installed", serviceName)
		}
		defer s.Close()
		err = s.Delete()
		if err != nil {
			return err
		}
		err = eventlog.Remove(serviceName)
		if err != nil {
			return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
		}
		log.Println("service was successfully uninstalled")
		os.Exit(0)
		return nil
	})
	b.Command("start", "starts windows service", func(runner *Runner, args Args, flags Flags) error {
		controlService(serviceName, svc.Stop, svc.Stopped) // In case service is already started
		if err := startService(serviceName); err != nil {
			return err
		}
		log.Println("service was successfully started")
		os.Exit(0)
		return nil
	})
	b.Command("stop", "stops windows service", func(runner *Runner, args Args, flags Flags) error {
		if err := controlService(serviceName, svc.Stop, svc.Stopped); err != nil {
			return err
		}
		<-time.After(time.Second * 2)
		log.Println("service was successfully stopped")
		os.Exit(0)
		return nil
	})
	b.Command("pause", "pauses windows service", func(runner *Runner, args Args, flags Flags) error {
		if err := controlService(serviceName, svc.Pause, svc.Paused); err != nil {
			return err
		}
		log.Println("service was successfully paused")
		os.Exit(0)
		return nil
	})
	b.Command("continue", "continues windows service", func(runner *Runner, args Args, flags Flags) error {
		if err := controlService(serviceName, svc.Continue, svc.Running); err != nil {
			return err
		}
		log.Println("service was successfully continued")
		os.Exit(0)
		return nil
	})
	return b
}

func startService(serviceName string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start() // Supports args
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

var elog debug.Log

var _ io.Writer = &ExecuteLogger{}

type ExecuteLogger struct {
	b *Builder
}

func (l *ExecuteLogger) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	defer func() {
		go func() {
			<-time.After(time.Second * 2)
			log.Println("graceful shutdown failed, hard shutdown pending")
			<-time.After(time.Millisecond * 100)
			os.Exit(0)
		}()
	}()
Loop:
	for {
		select {
		case <-l.b.runner.ctx.Done():
			break Loop
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Println("application has received a stop signal")
				break Loop
			default:
				log.Println(fmt.Sprintf("unexpected control request #%d", c))
				break Loop
			}
		}
	}
	l.b.runner.Stop()
	changes <- svc.Status{State: svc.StopPending}
	return
}

func (l *ExecuteLogger) Write(p []byte) (n int, err error) {
	if err := elog.Info(1, string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			return "", fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}
