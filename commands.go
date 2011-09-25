package main

import (
	"strings"
	"strconv"
	irc "cbeck/ircbot"
	"fmt"
	"io/ioutil"
	)

type commandFunc func([]string) []string
type command struct {
	raw string
	sender string
	source int
}

const (
	DefaultStopDelay = 5e9
)

var commandMap map[string]commandFunc = 
	map[string]commandFunc {
	"?" : helpCmd,
	"backup" : backupCmd,
	"ban" : banCmd,
	"pardon" : pardonCmd,
	"give" : giveCmd,
	"help" : helpCmd,
	"kick" : kickCmd,
	"list" : listCmd,
	"mapgen" : mapgenCmd,
	"restart" : restartCmd,
	"source" : sourceCmd,
	"start" : startCmd,
	"state" : stateCmd,
	"stop" : stopCmd,
	"tp" : tpCmd,
}

var commandHelpMap map[string]string = 
	map[string]string {
	"?" : "? [command]: If [command] is present, get usage information on that command, otherwise" +
		" display a list of available commands",

	"backup" : "backup [name]: Force the creation of a persistant backup.  If [name] is present," + 
		" the file will be named 'name.backup', otherwise it will be '<RFC3339 time>.backup'.",

	"ban" : "ban <name or ip> [duration]: Ban a player by ip or name.  If [duration] is present, " +
		"the ban will be lifted after than many minutes. " +
		"If no arguments are passed, get a list of currently banned players and IPs.",

	"pardon" : "pardon <name or ip>: Remove a player from the banned list by name or IP.",

	"give" : "give <player> <item id or name> [num]: Spawn <item> at <player>'s location.  If [num] " +
		"is present, spawn that many of <item>.  Some items may not be spawnable by name.",

	"help" :  "help [command]: If [command] is present, get usage information on that command, otherwise" +
		" display a list of available commands",,

	"kick" : "kick <player> [duration]: Kick <player> off the server.  Player will be able to rejoin" + 
		" immediatly unless [duration] is present, in which case they will be banned for that many minutes.",

	"list" : "list: List all players currently connected to the server.",

	"mapgen" : "mapgen: Force a run of the map generator.  If a mapgen is currently running, get an" +
		" estimate of its progress.",

	"restart" : fmt.Sprintf("restart [delay] [message]: Restart the server after issuing [message] and " +
		"waiting [delay] seconds.  If [delay] is not present, wait %d seconds.", defaultStopDelay / 1e9),

	"source" : "source: Get information on this bot's source code.",

	"start" : "start: Start the Minecraft server if it's stopped.",

	"state" : "state: Get information on the current server process.",

	"stop" : fmt.Sprintf("stop [delay] [message]: Stop the server after issuing [message] and waiting " +
		"[delay] seconds.  If [delay] is not present, wait %d seconds.", defaultStopDelay / 1e9),

	"tp" : "tp <player> <destination player>: Teleport <player> to <destination player>'s location.",
}


func directedIRC(cmd string, m *irc.Message) string {
	commands <- &command{cmd, m.GetSender(), SOURCE_IRC}
	return ""
}

func commandDispatch() {
	var reply []string

	for cmd := range commands {
		split := strings.Split(cmd.raw, " ")
		if len(split) < 1 {
			continue
		}

		f, exists := commandMap[split[0]]

		if !exists {
			reply = []string{"Unknown command: " + split[0]}
		} else if !allowed(cmd.sender, split[0], cmd.source) {
			reply = []string{cmd.sender + " is not allowed to invoke '" + split[0] + 
					"'. This incident will be reported."}
		
			logInfo.Printf("%s attempted '%s'\n", cmd.sender, cmd.raw)
		} else {
			reply = f(split[1:])
		}
		
		switch cmd.source {
		case SOURCE_MC:
			for _, s := range reply {
				server.In <- "say " + s
			}
		case SOURCE_IRC:
			for _, s := range reply {
				bot.Send(&irc.Message{
				Command : "PRIVMSG",
				Args : []string{config.IrcChan},
				Trailing : s,
				})		
			}
		}
	}
}

func allowed(sender, op string, source int) bool {
	//Is op allowed by default?
	if exists, allowed := config.defaultAccess[op]; exists && allowed {
		return true
	}

	switch source {
	case SOURCE_MC:
		sender = "mc:" + sender
	case SOURCE_IRC:
	
		//TODO: Make sure irc nick is actually ok 

		sender = "irc:" + sender
	}
	
	//If user is marked as part of any groups
	if levels, ok := config.accessLevelMembers[sender]; ok {
		for _, l := range levels {
			level := config.accessLevels[l]
			if exists, allowed := level[op]; exists && allowed {
				return true
			}
		} 
	}

	return false
}



func helpCmd(args []string) []string {
	var reply string
	if len(args) == 0 {
		reply = "Available commands: ?"
		for k := range commandMap {
			reply += ", " + k
		}
	} else if len(args) == 1 {
		reply, ok = commandHelpMap[args[0]]
		if !ok {
			reply = "Unknown command: " + args[0]
		}
	} else {
		reply = "Usage: " + commandHelpMap["help"]
	}

	return []string{reply}
}

func backupCmd(args []string) []string {
	return nil 
}

func banCmd(args []string) []string {
	return nil
}

func pardonCmd(args []string) []string {
	return nil
}

func giveCmd(args []string) []string {
	return nil 
}

func kickCmd(args []string) []string {
	return nil 
}


var listRegex *regexp.MustCompile(`\[INFO\] (Connected players: .*)`)
func listCmd(args []string) []string {
	server.In <- "list"

	for line := range commandResponse {
		if match := listRegex.FindStringSubmatch(line); if match != nil {
			return match[1:]
		}
	}

	return nil 
}

func mapgenCmd(args []string) []string {
	return nil 
}

func restartCmd(args []string) []string {
	return nil 
}

func sourceCmd(args []string) []string {
	return []string{"MCBot was written by Cory 'cbeck' Kolbeck.  Its source and license" +
			" information can be found at https://github.com/ckolbeck/mc-bot"}
}

func startCmd(args []string) []string {
	if len(args) > 0 {
		return []string{"'start' does not take any arguments"}
	}

	if err := server.Start(); err != nil {
		return []string{err.String()}
	}

	return []string{"Server started."}
}

func stateCmd(args []string) []string {
	pid, err := server.GetPID()
	if err != nil {
		return []string{err.String()}
	}
	switch config.HostOS {
	case "linux":
		raw, err := ioutil.ReadFile(fmt.Sprintf())






	case "windows":



	}

	return nil 
}

func stopCmd(args []string) []string {
	var delay int64
	var msg string

	if len(args) == 0 {
		delay = DefaultStopDelay
		msg = fmt.Sprintf("Stop command issued, going down in %d seconds.", delay / 1e9) 
	} else {
		if d, err := strconv.Atoi64(args[0]); err == nil {
			delay = d * 1e9
			msg = fmt.Sprintf("Stop command issued, going down in %d seconds.", delay / 1e9) 
		} else {
			delay = DefaultStopDelay
			msg = strings.Join(args, " ")
		}
	} 

	if err := server.Stop(delay, msg); err != nil {
		return []string{err.String()}
	}

	return []string{"Server stopped."}
}

func tpCmd(args []string) []string {
	return nil 
}
