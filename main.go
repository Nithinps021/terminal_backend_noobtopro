package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

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

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
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

	go func ()  {
		for{
			buf :=make([]byte,1024)
			read, err:=tty.Read(buf)
			if err!=nil{
				conn.WriteMessage(websocket.TextMessage,[]byte(err.Error()))
				l.WithError(err).Error(err.Error())
				return
			}
			conn.WriteMessage(websocket.BinaryMessage,buf[:read])

		}
	}()

	for{
		messageType,reader,err :=conn.NextReader()
		if err!=nil{
			l.WithError(err).Error("Not able to read next line")
			conn.WriteMessage(websocket.TextMessage,[]byte("Unable to read message typr from reader"))
			return
		}
		if messageType == websocket.TextMessage{
			l.Warn("Unexpected Message type")
			conn.WriteMessage(websocket.TextMessage, []byte("Unexpected text message"))
			continue
		}
		dataTypeBuf := make([]byte,1)
		read,err :=reader.Read(dataTypeBuf)
		if err!=nil{
			l.WithError(err).Error("Unable to read message type from reader")
			conn.WriteMessage(websocket.TextMessage, []byte("Unable to read message type from reader"))
			return
		}
		if read != 1 {
			l.WithField("bytes", read).Error("Unexpected number of bytes read")
			return
		}

		switch dataTypeBuf[0]{
			case 0 :
				copied, err :=io.Copy(tty,reader)
				if err!=nil {
					l.WithError(err).Errorf("Error after copying %d bytes", copied)
				}
			case 1 :
				decoder := json.NewDecoder(reader)
				resizeMessage := windowSize{}
				err := decoder.Decode(&resizeMessage)
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, []byte("Error decoding resize message: "+err.Error()))
					continue
				}
				log.WithField("resizeMessage", resizeMessage).Info("Resizing terminal")
				_, _, errno := syscall.Syscall(
					syscall.SYS_IOCTL,
					tty.Fd(),
					syscall.TIOCSWINSZ,
					uintptr(unsafe.Pointer(&resizeMessage)),
				)
				if errno != 0 {
					l.WithError(syscall.Errno(errno)).Error("Unable to resize terminal")
				}
			default:
				l.WithField("dataType", dataTypeBuf[0]).Error("Unknown data type")
		}
	}

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