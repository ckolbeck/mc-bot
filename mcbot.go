//Copyright 2010 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/ckolbeck/ircbot"
	"github.com/ckolbeck/mcserver"
	"log"
	"os"
	"os/exec"
)

var (
	bot     *ircbot.Bot
	server  *mcserver.Server
	config  *Config
	logErr  *log.Logger = log.New(os.Stderr, "[E] ", log.Ldate|log.Ltime)
	logInfo *log.Logger = log.New(os.Stdout, "[I] ", log.Ldate|log.Ltime)
)

func main() {
	var err error
	defer func() {
		if x := recover(); x != nil {
			fmt.Fprintf(os.Stderr, "Fatal error: %s\nExiting.", x)
			os.Exit(1)
		}
	}()

	confFile := flag.String("c", "./mcbot.conf", "The location of the configuration file to be used.")
	flag.Parse()

	if config, err = ReadConfig(*confFile); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	if bot, err = ircbot.NewBot(config.Nick, config.Pass, config.IrcDomain, config.IrcServer, config.IrcPort,
		config.SSL, config.AttnChar[0]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	if server, err = mcserver.NewServer(config.MCServerCommand.Command, config.MCServerCommand.Args,
		config.MCServerDir, logInfo, logErr); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	go commandDispatch()
	go readConsoleInput()
	go teeServerOutput()
	bot.SetPrivmsgHandler(directedIRC, echoIRCToServer)
	bot.JoinChannel(config.IrcChan, config.IrcChanKey)

	select {}
}

func copyWorld(worlddir, target string) error {
	var cmd string
	var args []string
	var err error

	switch config.HostOS {
	case "linux":
		cmd, err = exec.LookPath("rsync")
		if err == nil {
			args = []string{"-a", worlddir + "/", target}
		} else {
			cmd, err = exec.LookPath("cp")
			if err == nil {
				args = []string{"-r", "-p", "-u", worlddir, target}
			} else {
				return errors.New("Failed to find a suitable copy program (rsync, cp)")
			}
		}

	case "windows":
		cmd, err = exec.LookPath("copy")
		if err != nil {
			return errors.New("Failed to find copy program (copy)")
		}
		//This will do braindead things depending on wether dir exists
		args = []string{"/Y", worlddir, target}
	}

	command := exec.Command(cmd, args...)
	return command.Run()
}
