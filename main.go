package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"

	"github.com/kardianos/service"
)

const (
	ExitCodeNoServiceSystemAvailable = 60
	ExitCodeElevationCanceled        = 61
)

var (
	flagName        = flag.String("name", "", "name of the service to launch at startup")
	flagDescription = flag.String("description", "", "small description of what the service does")
	flagExecPath    = flag.String("exec", "", "absolute path of the program to execute at startup")
	flagUser        = flag.String("user", "", "id of the user to launch the service")
	flagUninstall   = flag.Bool("uninstall", false, "uninstall the service")
)

func init() {
	flag.BoolFunc("help", "prints program usage", func(s string) error {
		println("Utility program that helps creating or removing a system service for launching a program at startup.")
		println("\nFor installation, all flags except -uninstall are required.")
		println("For uninstallation, only -name and -uninstall flags are required.")
		return nil
	})
	flag.Parse()

	if !*flagUninstall {
		if slices.Contains([]string{*flagName, *flagDescription, *flagExecPath, *flagUser}, "") {
			flag.Usage()
			os.Exit(2)
		}
	} else if *flagName == "" {
		println("Service name is required for uninstallation")
		flag.Usage()
		os.Exit(2)
	}
}

func main() {
	if !isElevated() {
		elevate()
		return
	}

	s := launchService()

	if *flagUninstall {
		if !isInstalled(s) {
			println("service is not installed")
			return
		}
		err := s.Uninstall()
		if err != nil {
			fmt.Printf("Failed to uninstall service: %v\n", err)
			os.Exit(1)
		}
		println("service uninstalled successfully")
		return
	}

	if isInstalled(s) {
		println("service already installed")
		return
	}

	err := s.Install()
	if err != nil {
		fmt.Printf("Failed to install service: %v\n", err)
		os.Exit(1)
	}
	println("service installed and managed by: " + s.Platform())
}

func isInstalled(s service.Service) bool {
	_, err := s.Status()
	return err != service.ErrNotInstalled
}

func launchService() service.Service {
	cfg := &service.Config{
		Name:        *flagName,
		DisplayName: *flagName,
		Description: *flagDescription,
		Executable:  *flagExecPath,
		UserName:    *flagUser,
	}

	s, err := service.New(nil, cfg)
	if err == service.ErrNoServiceSystemDetected {
		println("no service manager available")
		os.Exit(ExitCodeNoServiceSystemAvailable)
	}
	return s
}

func isElevated() bool {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("net", "session")
		err := cmd.Run()
		return err == nil
	} else {
		return os.Geteuid() == 0
	}
}

func elevate() {
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable path: %v\n", err)
		os.Exit(1)
	}

	args := os.Args[1:]

	if runtime.GOOS == "windows" {
		exe := "powershell"
		command := fmt.Sprintf("Start-Process '%s' -ArgumentList '%s' -Verb RunAs",
			executable, strings.Join(args, "' '"))

		cmd := exec.Command(exe, "-Command", command)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Run()
		if err != nil {
			fmt.Printf("Failed to elevate privileges: %v\n", err)
			os.Exit(ExitCodeElevationCanceled)
		}
	} else {
		args = append([]string{executable}, args...)
		cmd := exec.Command("sudo", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		fmt.Printf("Elevating privileges: %s\n", strings.Join(cmd.Args, " "))

		err = cmd.Run()
		if err != nil {
			fmt.Printf("Failed to elevate privileges: %v\n", err)
			os.Exit(ExitCodeElevationCanceled)
		}
	}
}
