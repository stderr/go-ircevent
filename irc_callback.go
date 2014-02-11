package irc

import (
	"strconv"
	"strings"
	"time"
	"crypto/sha1"
	"fmt"
	"reflect"
	"math/rand"
)

func (irc *Connection) AddCallback(eventcode string, callback func(*Event)) string {
	eventcode = strings.ToUpper(eventcode)

	if _, ok := irc.events[eventcode]; !ok {
		irc.events[eventcode] = make(map[string]func(*Event))
	}
	h := sha1.New()
	rawId := []byte(fmt.Sprintf("%v%d", reflect.ValueOf(callback).Pointer(), rand.Int63()))
	h.Write(rawId)
	id := fmt.Sprintf("%x", h.Sum(nil))
	irc.events[eventcode][id] = callback
	return id
}

func (irc *Connection) RemoveCallback(eventcode string, i string) bool {
	eventcode = strings.ToUpper(eventcode)

	if event, ok := irc.events[eventcode]; ok{
		if _, ok := event[i]; ok {
			delete(irc.events[eventcode], i)
			return true
		}
		irc.Log.Printf("Event found, but no callback found at id %s\n", i)
		return false
	}

	irc.Log.Println("Event not found")
	return false
}

func (irc *Connection) ReplaceCallback(eventcode string, i string, callback func(*Event)) {
	eventcode = strings.ToUpper(eventcode)

	if event, ok := irc.events[eventcode]; ok {
		if _, ok := event[i]; ok {
			event[i] = callback
			return
		}
		irc.Log.Printf("Event found, but no callback found at id %s\n", i)
	}
	irc.Log.Printf("Event not found. Use AddCallBack\n")
}

func (irc *Connection) RunCallbacks(event *Event) {
	if event.Code == "PRIVMSG" && len(event.Message) > 0 && event.Message[0] == '\x01' {
		event.Code = "CTCP" //Unknown CTCP

		if i := strings.LastIndex(event.Message, "\x01"); i > -1 {
			event.Message = event.Message[1:i]
		}

		if event.Message == "VERSION" {
			event.Code = "CTCP_VERSION"

		} else if event.Message == "TIME" {
			event.Code = "CTCP_TIME"

		} else if event.Message[0:4] == "PING" {
			event.Code = "CTCP_PING"

		} else if event.Message == "USERINFO" {
			event.Code = "CTCP_USERINFO"

		} else if event.Message == "CLIENTINFO" {
			event.Code = "CTCP_CLIENTINFO"

		} else if event.Message[0:6] == "ACTION" {
			event.Code = "CTCP_ACTION"
			event.Message = event.Message[7:]

		}
	}

	if callbacks, ok := irc.events[event.Code]; ok {
		if irc.VerboseCallbackHandler {
			irc.Log.Printf("%v (%v) >> %#v\n", event.Code, len(callbacks), event)
		}

		for _, callback := range callbacks {
			go callback(event)
		}
	} else if irc.VerboseCallbackHandler {
		irc.Log.Printf("%v (0) >> %#v\n", event.Code, event)
	}

	if callbacks, ok := irc.events["*"]; ok {
		if irc.VerboseCallbackHandler {
			irc.Log.Printf("Wildcard %v (%v) >> %#v\n", event.Code, len(callbacks), event)
		}

		for _, callback := range callbacks {
			go callback(event)
		}
	}
}

func (irc *Connection) setupCallbacks() {
	irc.events = make(map[string]map[string]func(*Event))

	//Handle ping events
	irc.AddCallback("PING", func(e *Event) { irc.SendRaw("PONG :" + e.Message) })

	//Version handler
	irc.AddCallback("CTCP_VERSION", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01VERSION %s\x01", e.Nick, irc.Version)
	})

	irc.AddCallback("CTCP_USERINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01USERINFO %s\x01", e.Nick, irc.user)
	})

	irc.AddCallback("CTCP_CLIENTINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01CLIENTINFO PING VERSION TIME USERINFO CLIENTINFO\x01", e.Nick)
	})

	irc.AddCallback("CTCP_TIME", func(e *Event) {
		ltime := time.Now()
		irc.SendRawf("NOTICE %s :\x01TIME %s\x01", e.Nick, ltime.String())
	})

	irc.AddCallback("CTCP_PING", func(e *Event) { irc.SendRawf("NOTICE %s :\x01%s\x01", e.Nick, e.Message) })

	irc.AddCallback("437", func(e *Event) {
		irc.nickcurrent = irc.nickcurrent + "_"
		irc.SendRawf("NICK %s", irc.nickcurrent)
	})

	irc.AddCallback("433", func(e *Event) {
		if len(irc.nickcurrent) > 8 {
			irc.nickcurrent = "_" + irc.nickcurrent

		} else {
			irc.nickcurrent = irc.nickcurrent + "_"
		}
		irc.SendRawf("NICK %s", irc.nickcurrent)
	})

	irc.AddCallback("PONG", func(e *Event) {
		ns, _ := strconv.ParseInt(e.Message, 10, 64)
		delta := time.Duration(time.Now().UnixNano() - ns)
		if irc.Debug {
			irc.Log.Printf("Lag: %vs\n", delta)
		}
	})

	irc.AddCallback("NICK", func(e *Event) {
		if e.Nick == irc.nick {
			irc.nickcurrent = e.Message
		}
	})

	irc.AddCallback("001", func(e *Event) {
		irc.nickcurrent = e.Arguments[0]
	})
}
