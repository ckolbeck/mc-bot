//Copyright 2010 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package mcserver

import (
	"sync"
	"os"
	"exec"
	"time"
	"syscall"
	"fmt"
	"io"
	"log"
)

type Server struct {
	*exec.Cmd
	mutex *sync.Mutex
	running bool
	dir string
	loginfo *log.Logger
	logerr *log.Logger
}

const STOP_TIMEOUT = 5e9

func StartServer(dir string, loginfo, logerr *log.Logger) (*Server, os.Error) {
	proc, err := exec.Run("/usr/bin/java", []string{"-Xms1024M", "-Xmx1024M", "-jar", "Minecraft_Mod.jar", "nogui"},
		nil, dir, exec.Pipe, exec.Pipe, exec.MergeWithStdout)

	if err != nil {
		return nil, err
	}

	return &Server{proc, &sync.Mutex{}, true, dir, loginfo, logerr}, nil
}

func (s *Server) Stop(delay int64, msg string) {
	if s == nil || !s.running {
		return
	}

	s.mutex.Lock()
	s.running = false
	s.Stdin.WriteString("\nsave-all\n")
	s.Stdin.WriteString("say " + msg + "\n")
	if delay < 0 {
		time.Sleep(10e9)
	} else {
		time.Sleep(delay)
	} 

	s.Stdin.WriteString("stop\n")
	
	done, alarm := make(chan int), make(chan int)

	go func() {
		s.Wait(0)
		_ = done <- 1
	}()

	go timeout(STOP_TIMEOUT, alarm)

	select {
	case <-done:
	case <-alarm:
		syscall.Kill(s.Pid, 9)
	}
	s.running = false
	s.mutex.Unlock()
}

func (s *Server) BackupState(filename string) (err os.Error) {
	if s == nil || !s.running {
		return os.NewError("Unable to perform backup, server not running")
	}

	s.mutex.Lock()
	s.Stdin.WriteString("say Backup in progress...\n")
	s.Stdin.WriteString("save-off\n")

	time.Sleep(5e9) //Let save-off go through
	//tar -czf mcServerBackups/$1 banned-ips.txt server.log server.properties banned-players.txt ops.txt world
	
	bu, err := exec.Run("/bin/tar", 
		[]string{" ", "-czf", "mcServerBackups/" + filename, "banned-ips.txt", "server.log", "server.properties", "banned-players.txt", "ops.txt", "world"},
		nil, s.dir, exec.DevNull, exec.Pipe, exec.MergeWithStdout)

	if bu != nil {
		go io.Copy(os.Stdout, bu.Stdout)
		wm, _ := bu.Wait(os.WSTOPPED)
		if ex := wm.ExitStatus(); ex != 0 {
			err = os.NewError(fmt.Sprintf("Backup command returned errorcode: %d", ex))
		}
	}

	s.Stdin.WriteString("save-on\n")
	s.mutex.Unlock()
	
	return
}

func (s *Server) IsRunning() bool {
	return s != nil && s.running
}

func timeout(ns int64, alarm chan int) {
	time.Sleep(ns)
	_ = alarm <- 1
}