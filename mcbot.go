//Copyright 2010 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package main

import (
	"cbeck/ircbot"
	"./mcserver"
	"time"
	"strings"
	"os"
	"fmt"
	"log"
	"io"
	"io/ioutil"
	"regexp"
	"strconv"
	"bufio"
)

var bot *ircbot.Bot
var server *mcserver.Server
var trusted map[string]bool = map[string]bool{"cbeck":true}
var ignored map[string]bool = map[string]bool{}
var lastList chan string = make(chan string, 1)
var listReq bool

//If false, only trusted people can issue -any- commands
var freeForAll bool = true

const network = "irc.cat.pdx.edu"
const admin = "cbeck"
const mcDir = "/disk/trump/cbeck"

func main() {
	for {
		session()
		time.Sleep(10e9)
	}
}

func session() {
	defer ircbot.RecoverWithTrace()
	bot = ircbot.NewBot("MC-Bot", '!')

	bot.SetPrivmsgHandler(parseCommand, echoChat)
	_, e := bot.Connect(network, 6667, []string{"#minecraft"})

	if e != nil {
		panic(e.String())
	}
	
	server, e = mcserver.StartServer(mcDir) 
	
	if e != nil {
		log.Stderr("[E] Error creating server")
		panic(e.String())
	}
	
	go autoBackup(server)
	go monitorOutput(server)
	go io.Copy(server.Stdin, os.Stdin)
	
	defer func(s *mcserver.Server) {
		s.Stop(1e9,"Crash Intercepted, server going down NOW")
	}(server)

	select {}
}

var sanitizeRegex *regexp.Regexp = regexp.MustCompile(`[^ -~]`)

func parseCommand(c string, m *ircbot.Message) string {
	sender := m.GetSender()
	
	if (ignored[sender] && sender != admin) || (!freeForAll && !trusted[sender]) {
		return ""
	}

	c = sanitizeRegex.ReplaceAllString(c, "_")
		
	var args []string
	split := strings.Split(strings.TrimSpace(c), " ", 2)
	command := strings.ToLower(split[0])
	if len(split) > 1 {
		args = strings.Split(split[1], " ", -1)
	}

	switch command {
	case "give":
		return give(args)
	case "restart":
		return restart(sender)
	case "backup":
		return backup(args)
	case "state":
		return state()
	case "stop":
		return stop(sender)
	case "halt" :
		if !trusted[sender] {
			return ""
		}	
		server.Stop(1e9,"Server going down NOW!")
		return "Server halted"
	case "tp" :
		return tp(args)
	case "ignore" : 
		return ignore(args, sender)
	case "trust" :
		return trust(args, sender)
	case "list" :
		listReq = true
		server.Stdin.WriteString("\nlist\n")
		return <-lastList
	case "ffa" :
		if !trusted[sender] {
			return ""
		}
		freeForAll = !freeForAll
	case "help" :
		return "give | restart | list | backup | state | stop | tp | source | help"

	case "mc-bot", "source" : 
		return "MC-Bot was written by Cory Kolbeck. Its source can be found at http://github.com/ckolbeck/mc-bot"
	}

	return "Huh?"
}


func echoChat(c string, m *ircbot.Message) string {
	c = sanitizeRegex.ReplaceAllString(c, "_")

	fmt.Fprintf(server.Stdin, "say <%s> %s\n", m.GetSender(), c)

	return ""
}

func stop(sender string) string {
	if !trusted[sender] {
		return ""
	}

	if !server.IsRunning() {
		return "The server is not currently running"
	}

	server.Stop(10e9, fmt.Sprintf("Server halt requested by %s, going down in 10s\n", sender))
	server = nil
	return "Server halted."
}

func give(args []string) string {
	if !server.IsRunning() {
		return "The server is not currently running"
	}

	if len(args) == 2 { 
		fmt.Fprintf(server.Stdin, "give %s %s %s\n", args[0], args[1], "1")
	} else if len(args) == 3 {
		fmt.Fprintf(server.Stdin, "give %s %s %s\n", args[0], args[1], args[2])
	} else {
		return "Expected format: `give <playername> <objectid> [num]`"
	}
	
	return ""
}

func restart(sender string) string {
	if !trusted[sender] {
		return ""
	}

	var err os.Error

	server.Stop(10e9, fmt.Sprintf("Server restart requested by %s, going down in 10s\n", sender))
	server, err = mcserver.StartServer("/disk/trump/cbeck")

	if err != nil {
		return "Could not start server: " + err.String()
	}

	go autoBackup(server)
	go monitorOutput(server)
	go io.Copy(server.Stdin, os.Stdin)

	return "Server restarted"
}

func backup(args []string) string {
	if !server.IsRunning() {
		return "The server is not currently running"
	}
	
	var bkfile string

	if len(args) > 0 {
		bkfile = args[0] + ".tgz"
	} else {
		bkfile = time.LocalTime().Format("2006-01-02T15_04_05") + ".tgz"
	}

	err := server.BackupState(bkfile)

	if err != nil {
		return "Error attempting to perform backup: " + err.String()
	}

	return "Backup finished"
}

func state() string {
	if !server.IsRunning() {
		return fmt.Sprintf("The server is not currently running")
	}

	usage, err := getMemUsage()

	if err != nil {
		return err.String()
	}

	return fmt.Sprintf("Server is up and currently using %dK virtual memory", usage)
}

var memRegex *regexp.Regexp = regexp.MustCompile("VmSize:[^0123456789]*([0123456789]+)")

func getMemUsage() (int, os.Error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", server.Pid), os.O_RDONLY, 0444)
	
	if err != nil {
		return -1, os.NewError("Error opening status file: " + err.String())
	}

	defer f.Close()

	raw, err := ioutil.ReadAll(f)
	
	if err != nil {
		return -1, os.NewError("Error reading status file")
	}
	
	mtch := memRegex.FindSubmatch(raw)

	if mtch == nil {
		return -1, os.NewError("Error in regexp parsing of status file")
	}

	usage, err := strconv.Atoi(string(mtch[1]))

	if err != nil {
		return -1, os.NewError("Error parsing status file")
	}

	return usage, nil
}

func tp(args []string) string {
	if len(args) == 2 {
		fmt.Fprintf(server.Stdin, "tp %s %s\n", args[0], args[1])
	} else {
		return "Expected format: `tp <player> <target-player>`"
	} 

	return ""
}

func ignore(args []string, sender string) string {
	if !trusted[sender] {
		return ""
	}
	
	ign := "Ignoring: "
	unign := "Unignoring: "
	
	for _, i := range args {
		if (!trusted[i] || sender == admin) && i != admin {
			if ignored[i] {
				unign += i + " "
				ignored[i] = false, false
			} else {
				ign += i + " "
				ignored[i] = true
			}
		} 
	}

	return ign + unign
}

func trust(args []string, sender string) string {
	if sender != admin {
		return ""
	}
	
	trst := "Trusting: "
	untrst := "Untrusting: "
	
	for _, i := range args {
		if i != admin {
			if trusted[i] {
				untrst += i + " "
				trusted[i] = false, false
			} else {
				trst += i + " "
				trusted[i] = true
			}
		}
	}

	return trst + untrst
}

func autoBackup(s *mcserver.Server) {
	tick := time.Tick(3610e9)
	for s.IsRunning() {
		t := time.LocalTime()
		s.BackupState(fmt.Sprintf("%d.tgz", t.Hour))
		<-tick
	}
}

var listRegex *regexp.Regexp = regexp.MustCompile(`Connected players:.*`)
var msgRegex *regexp.Regexp = regexp.MustCompile(`\[INFO\] <[a-zA-Z0-9_]+>`)

func monitorOutput(s *mcserver.Server) {
	defer ircbot.RecoverWithTrace()

	in := bufio.NewReader(s.Stdout)

	for str, err := in.ReadString('\n'); s.IsRunning() && err == nil; str, err = in.ReadString('\n') {
		if listReq && listRegex.MatchString(str) {
			lastList <- str[27:]
			listReq = false
		} else if msgRegex.MatchString(str) {
			bot.Send(network, &ircbot.Message{
			Command : "PRIVMSG",
			Args : []string{"#minecraft"},
			Trailing : str[27:],
			})
		}

		log.Stdout(str)
	}
}
