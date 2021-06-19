package main

import (
	"time"

	//"github.com/gorilla/websocket"
	"golang.org/x/net/websocket"
	"io/ioutil"
	"net/http"
)
var webServer WebServer

const(
	//GET
	CMD_GET_POINTS_FROM_DB="getPointsFromDB"
	CMD_GET_CHANNELS_FROM_DB="getChannelsFromDB"
	CMD_GET_RTUS_FROM_DB="getRtusFromDB"
	CMD_GET_DATABASELIST="getDatabaseList"
	CMD_GET_NODE_LIST="getNodeList"
	CMD_GET_POINTS_FROM_RTD="getPointsFromRTD"
	//PUT
)
type WebServer struct {
	sentToBus chan *message
	receiveFromBus chan *message
}
func newMsgFromWebToBus()*message{
	var msg message
	msg.SrcNodeID=BUS.LocalNodeID
	msg.SrcAppID=MODULAR_ID_WEB
	msg.DesNodeID=BUS.LocalNodeID
	msg.DesAppID=MODULAR_ID_BUS
	msg.FunctionCode=7
	msg.Para=0
	return &msg
}

func (webserver *WebServer)WebInit(bus *Bus){
	webserver.sentToBus=bus.ToBusChanMap[MODULAR_ID_WEB]
	webserver.receiveFromBus=bus.FromBusChanMap[MODULAR_ID_WEB]
	http.HandleFunc("/database/points",pageDatabasePoints)
	http.HandleFunc("/database/channels",pageDatabaseChannels)
	http.HandleFunc("/database/rtus",pageDatabaseRtus)
	http.HandleFunc("/rtd/points",pageRTDPoints)
	http.Handle("/websocket",websocket.Handler(wsHandler))
}
func pageDatabasePoints(w http.ResponseWriter,r *http.Request){
	concent,err:=ioutil.ReadFile("./web/html/databasePoints.html")
	LogIt("读取html失败",err,LOG_LEVEL_ERROR)
	w.Write(concent)
}
func pageDatabaseChannels(w http.ResponseWriter,r *http.Request){
	concent,err:=ioutil.ReadFile("./web/html/databaseChannels.html")
	LogIt("读取html失败",err,LOG_LEVEL_ERROR)
	w.Write(concent)
}
func pageDatabaseRtus(w http.ResponseWriter,r *http.Request){
	concent,err:=ioutil.ReadFile("./web/html/databaseRtus.html")
	LogIt("读取html失败",err,LOG_LEVEL_ERROR)
	w.Write(concent)
}
func pageRTDPoints(w http.ResponseWriter,r *http.Request){
	concent,err:=ioutil.ReadFile("./web/html/rTDPoints.html")
	LogIt("读取html失败",err,LOG_LEVEL_ERROR)
	w.Write(concent)
}

func wsHandler(w *websocket.Conn){

	for {
		wsMessage := make([]byte, 1024)
		len, err := w.Read(wsMessage)
		if err!=nil{
			_ = w.Close()
			return
		}
		LogIt("接收ws消息失败", err, LOG_LEVEL_ERROR)
		msg := newMsgFromWebToBus()
		msg.Data = wsMessage[:len]

		webServer.sentToBus <- msg
		timer := time.NewTimer(5*time.Second)
		select {
		case <-timer.C:
			continue
		case replyMsg := <-webServer.receiveFromBus:
			w.Write(replyMsg.Data)

		}


	}
}
