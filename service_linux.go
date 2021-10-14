//go:build linux

package cli

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"
)

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
	return "", err
}

// Daemonize makes the application work as a linux/windows service
func (b *Builder) Daemonize(serviceName string, serviceDescription string) *Builder {
	if b.daemoize {
		return b
	}
	b.daemoize = true
	exepath, err := exePath()
	if err != nil {
		panic(err)
	}
	serviceNameDashedUnderscored := strings.ToLower(strings.ReplaceAll(serviceName, " ", "-"))
	systemdServicePath := filepath.Join("/etc/systemd/system", serviceNameDashedUnderscored+".service")

	existsService := func() bool {
		_, err := os.Stat(systemdServicePath)
		return err == nil
	}

	enableService := func() error {
		cmd := exec.Command("systemctl", "enable", serviceNameDashedUnderscored)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return fmt.Errorf("already enabled")
		}
		return nil
	}

	disableService := func() error {
		cmd := exec.Command("systemctl", "disable", serviceNameDashedUnderscored)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return fmt.Errorf("already disabled")
		}
		return nil
	}

	statusService := func() (enabled bool, running bool, err error) {
		cmd := exec.Command("systemctl", "is-active", serviceNameDashedUnderscored)
		out, _ := cmd.CombinedOutput()
		if len(out) == 0 {
			return false, false, fmt.Errorf("no output from systemctl")
		}
		if strings.TrimSpace(string(out)) == "active" {
			running = true
		}
		cmd = exec.Command("systemctl", "is-enabled", serviceNameDashedUnderscored)
		out, _ = cmd.CombinedOutput()
		if len(out) == 0 {
			return false, false, fmt.Errorf("no output from systemctl")
		}
		if strings.TrimSpace(string(out)) == "enabled" {
			enabled = true
		}
		return enabled, running, nil
	}

	installService := func() error {
		funcMap := template.FuncMap{}
		tmp, err := template.New("systemd").Funcs(funcMap).Parse(systemdTemplate)
		if err != nil {
			return err
		}

		info, err := os.Stat(exepath)
		if err != nil {
			panic(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		uid := stat.Uid
		gid := stat.Gid
		u := strconv.FormatUint(uint64(uid), 10)
		g := strconv.FormatUint(uint64(gid), 10)
		usr, err := user.LookupId(u)
		if err != nil {
			panic(err)
		}
		group, err := user.LookupGroupId(g)
		if err != nil {
			panic(err)
		}

		var bts bytes.Buffer
		var templateData struct {
			Description             string
			BinaryPath              string
			User                    string
			Group                   string
			CurrentWorkingDirectory string
		}
		templateData.Description = serviceName + " - " + serviceDescription
		templateData.User = usr.Name
		templateData.Group = group.Name
		templateData.BinaryPath = exepath
		templateData.CurrentWorkingDirectory = filepath.Dir(exepath)
		if err := tmp.Execute(&bts, templateData); err != nil {
			return err
		}
		ioutil.WriteFile(systemdServicePath, bts.Bytes(), 0664)
		return nil
	}

	removeService := func() error {
		return os.Remove(systemdServicePath)
	}

	stopService := func() error {
		cmd := exec.Command("systemctl", "stop", serviceNameDashedUnderscored)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return fmt.Errorf("already stopped")
		}
		return nil
	}

	startService := func() error {
		_, running, err := statusService()
		if err != nil {
			return err
		} else if running {
			return fmt.Errorf("already started")
		}
		cmd := exec.Command("systemctl", "start", serviceNameDashedUnderscored)
		_, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Println("start cmd failed")
			return err
		}
		<-time.After(time.Second)
		_, running, err = statusService()
		if err != nil {
			return err
		} else if !running {
			return fmt.Errorf("failed to start service (was not running one second after start-up)")
		}
		return nil
	}
	b.Command("install", "installs systemd service", func(c *Runner, args Args, flags Flags) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("you need root priviledges to install this service")
		}
		_, err := os.Stat("/etc/systemd/system")
		if err != nil {
			return fmt.Errorf("/etc/systemd/system folder missing")
		}
		if existsService() {
			disableService()
			stopService()
			if err := removeService(); err != nil {
				return err
			}
		}
		if err := installService(); err != nil {
			return err
		}
		if err := enableService(); err != nil {
			return err
		}
		if err := startService(); err != nil {
			return err
		}
		log.Println("service was successfully installed and started")
		os.Exit(0)
		return nil
	})
	b.Command("remove", "uninstalls systemd service", func(c *Runner, args Args, flags Flags) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("you need root priviledges to remove this service")
		}
		if existsService() {
			disableService()
			stopService()
			if err := removeService(); err != nil {
				return err
			}
			log.Println("service was successfully uninstalled")
		} else {
			return fmt.Errorf("service %s is not installed", serviceNameDashedUnderscored)
		}
		os.Exit(0)
		return nil
	})
	b.Command("enable", "enables automatic start of systemd service", func(c *Runner, args Args, flags Flags) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("you need root priviledges to enable this service")
		}
		if !existsService() {
			return fmt.Errorf("service %s is not installed", serviceNameDashedUnderscored)
		}
		if err := enableService(); err != nil {
			return err
		}
		log.Println("automatic start of service was successfully enabled")
		os.Exit(0)
		return nil
	})
	b.Command("disable", "disables automatic start of systemd service", func(c *Runner, args Args, flags Flags) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("you need root priviledges to disable this service")
		}
		if !existsService() {
			return fmt.Errorf("service %s is not installed", serviceNameDashedUnderscored)
		}
		if err := disableService(); err != nil {
			return err
		}
		log.Println("automatic start of service was successfully disabled")
		os.Exit(0)
		return nil
	})
	b.Command("start", "starts systemd service", func(c *Runner, args Args, flags Flags) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("you need root priviledges to start this service")
		}
		if !existsService() {
			return fmt.Errorf("service %s is not installed", serviceNameDashedUnderscored)
		}
		if err := startService(); err != nil {
			return err
		}
		log.Println("service was successfully started")
		os.Exit(0)
		return nil
	})
	b.Command("stop", "stops systemd service", func(c *Runner, args Args, flags Flags) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("you need root priviledges to stop this service")
		}
		if !existsService() {
			return fmt.Errorf("service %s is not installed", serviceNameDashedUnderscored)
		}
		if err := stopService(); err == nil {
			log.Println("service was successfully stopped")
		} else if err.Error() == "already stopped" {
			log.Println("service was already stopped")
		} else {
			return err
		}
		os.Exit(0)
		return nil
	})
	b.Command("status", "checks systemd service status", func(c *Runner, args Args, flags Flags) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("you need root priviledges to status this service")
		}
		if !existsService() {
			return fmt.Errorf("service %s is not installed", serviceNameDashedUnderscored)
		}
		enabled, running, err := statusService()
		if err != nil {
			return err
		}
		log.Printf("enabled: %t, running: %t", enabled, running)
		os.Exit(0)
		return nil
	})
	return b
}

const systemdTemplate = `
[Unit]
Description={{ .Description }}
ConditionPathExists={{ .BinaryPath }}

[Service]
Type=simple

User={{ .User }}
Group={{ .Group }}

WorkingDirectory={{ .CurrentWorkingDirectory }}

Restart=always
RestartSec=3

ExecStart={{ .BinaryPath }}

[Install]
WantedBy=multi-user.target
`
