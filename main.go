package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type windowSize struct{
	Rows uint `json:"rows"`
	Cols uint `json:"cols"`
	X uint16
	Y uint16
}

var WebsocketMessageType = map[int]string{
	websocket.BinaryMessage: "binary",
	websocket.TextMessage:   "text",
	websocket.CloseMessage:  "close",
	websocket.PingMessage:   "ping",
	websocket.PongMessage:   "pong",
}
type TTYSize struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
	X    uint16 `json:"x"`
	Y    uint16 `json:"y"`
}


var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {return true},
}
func handleWebSocket(w http.ResponseWriter, r *http.Request){
	l:=log.WithField("RemoteAddress", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w,r,nil)
	if err!=nil{
		l.WithError(err).Error("Unable to Upgrade")
	}

	cmd := exec.Command("/bin/bash","-l")
	cmd.Env =append(os.Environ(),"Term=xterm")

	tty, err := pty.Start(cmd);
	if err!= nil{
		l.WithError(err).Error("Unable to start pty/cmd")
		conn.WriteMessage(websocket.TextMessage,[]byte(err.Error()))
	}

	defer func(){
		cmd.Process.Kill()
		cmd.Process.Wait()
		tty.Close()
		conn.Close()
	}()

	var waiter sync.WaitGroup
	waiter.Add(1)
	go func ()  {
		for{
			buf :=make([]byte,1024)
			read, err:=tty.Read(buf)
			if err!=nil{
				conn.WriteMessage(websocket.TextMessage,[]byte(err.Error()))
				l.WithError(err).Error(err.Error())
				waiter.Done()
				return
			}
			conn.WriteMessage(websocket.BinaryMessage,buf[:read])
		}
	}()

	go func ()  {
		for{
			messageType, data, err :=conn.ReadMessage()
			if(err!=nil){
				l.WithError(err).Error("failed to get next reader")
				return
			}
			dataLength :=len(data)
			dataBuffer := bytes.Trim(data,"\x00")
			dataType,ok := WebsocketMessageType[messageType]
			if !ok {
				dataType="unknwon"
			}
			l.Info("Type of message is %s",dataType)
			if dataLength ==-1 {
				l.Warn("Failed to get correct number of bytes read")
				continue
			}
			if messageType == websocket.BinaryMessage {
				if dataBuffer[0]==1{
					ttySize :=&TTYSize{}
					resizeMessage := bytes.Trim(dataBuffer[1:], " \n\r\t\x00\x01")
					if err := json.Unmarshal(resizeMessage, ttySize); err != nil {
						l.Warnf("failed to unmarshal received resize message '%s': %s", string(resizeMessage), err)
						continue
					}
					l.Info("resizing tty to use %v rows and %v columns...", ttySize.Rows, ttySize.Cols)
					if err := pty.Setsize(tty, &pty.Winsize{
						Rows: ttySize.Rows,
						Cols: ttySize.Cols,
					}); err != nil {
						l.Warnf("failed to resize tty, error: %s", err)
					}
					continue
				}
			}
			bytesWritten, err := tty.Write(dataBuffer)
			l.Info("bytes written ",bytesWritten)
			if err != nil {
				l.Warn(fmt.Sprintf("failed to write %v bytes to tty: %s", len(dataBuffer), err))
				continue
			}
			l.Tracef("%v bytes written to tty...", bytesWritten)
		}
	}()
	waiter.Wait()
 }


func main(){ 
	var listen = flag.String("listen", "127.0.0.1:8080", "Host:port to listen on")
	var assetsPath = flag.String("assets", "./assets", "Path to assets")

	flag.Parse()

	r := mux.NewRouter()

	r.HandleFunc("/term",handleWebSocket)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(*assetsPath)))

	log.Info("Demo Websocket/Xterm terminal")

	if !(strings.HasPrefix(*listen, "127.0.0.1") || strings.HasPrefix(*listen, "localhost")) {
		log.Warn("Danger Will Robinson - This program has no security built in and should not be exposed beyond localhost, you've been warned")
	}

	if err := http.ListenAndServe(*listen, r); err != nil {
		log.WithError(err).Fatal("Something went wrong with the webserver")
	}
	 

}