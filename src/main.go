package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
)

///////////////////////////////////////////////////////////////////////////
//全局参数区域
var Cfg Config

var BUS Bus
//log
var logger *Logger
//本地模块发送给bus的信息通道
var LocalToBus map[int]chan *message
//Bus发送给本地模块的信息通道
var LocalFromBus map[int]chan *message

var Timer *TimeFactory
//应用号
const(
	BUS_ID=0
	WEB_ID=1
	DATABASE_ID=2
	DATAPROCESS=3
	PROTOCOL_ID=4
)


///////////////////////////////////////////////////////////////////////////
func main(){
	Timer=NewFactory()
	go Timer.Run()
	logger=NewLogger(LOG_LEVEL_FATAL+LOG_LEVEL_ERROR+LOG_LEVEL_WARN+LOG_LEVEL_INFO+LOG_LEVEL_DEBUG+LOG_LEVEL_ALL,100000000)
	logger.Init()
	go logger.Run()
	LogIt("开始读取config文件",nil,LOG_LEVEL_INFO)
	BUS.InitConfig(&Cfg)
	LogIt("开始运行bus",nil,LOG_LEVEL_INFO)
	go RunBus(&BUS)
	LogIt("开始将磁盘数据库信息读入内存库",nil,LOG_LEVEL_INFO)


	RealTimeDatabase.InitMemoryDatabase(&Cfg)
	LogIt("开始启动webServer",nil,LOG_LEVEL_INFO)
	webServer.WebInit(&BUS)
	go http.ListenAndServe(Cfg.WebServerAddr,nil)
	fmt.Sprintf("准备工作结束请，开始访问http://%s/database/channels进行配置",Cfg.WebServerAddr)
	select{

	}
}