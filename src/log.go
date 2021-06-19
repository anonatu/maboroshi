package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)
//var logger *Logger

const(
	LOG_LEVEL_ALL=1
	LOG_LEVEL_DEBUG=2
	LOG_LEVEL_INFO=4
	LOG_LEVEL_WARN=8
	LOG_LEVEL_ERROR=16
	LOG_LEVEL_FATAL=32
)

//常量获取日志等级字符串，这个需要在init函数中完成初始化
var logClass map[int]string
var mutex sync.Mutex

//单条log信息
type logMessage struct{
	Message string
	Flag int
	Prefix string
	Modular string
	Line int
	Err error
}

type Logger struct{
	ModularList []string
	LogFile map[string]*os.File
	LogWriter map[string]io.Writer
	TimePrefix string
	//用于控制哪类log可以记录
	ConsoleFlag int
	CallerDept int
	LogChan chan *logMessage
	wg sync.WaitGroup
	LogClasss map[int]string
	//log文件的切割尺寸
	MaxSizeOfLogFile int64


}


func NewLogger(cf int,fileSize int64)*Logger{
	return &Logger{
		//应可配置
		ModularList:[]string{"ALL","bus","webserver","modbusTcpServer","database","main"},
		LogFile: make(map[string]*os.File),
		LogWriter: make(map[string]io.Writer),
		TimePrefix: "2006-01-02 15:04:05.000000000",
		ConsoleFlag: cf,
		CallerDept: 2,
		LogChan: make(chan *logMessage,500),
		MaxSizeOfLogFile: fileSize,
	}
}
//初始化logger，自动创建出all日志文件，如果all位为0，则全部写入标准输出

func (logger *Logger)Init(){
	for i:=0;i<len(logger.ModularList);i++ {

		f1, err := os.OpenFile("./log/"+logger.ModularList[i]+".log", os.O_APPEND|os.O_CREATE, 0755)
		logger.LogFile[logger.ModularList[i]] = f1

		if err != nil {
			fmt.Println("打开或创建all日志文件失败：", err)
		}
		logger.LogWriter[logger.ModularList[i]] = f1
	}
	if logger.ConsoleFlag&1>0 {
		logger.LogFile["ALL"].Close()
		logger.LogWriter["ALL"] = os.Stdout
	}
	logger.LogClasss=map[int]string{
		LOG_LEVEL_ALL:"ALL",
		LOG_LEVEL_DEBUG:"DEBUG",
		LOG_LEVEL_INFO:"INFO",
		LOG_LEVEL_WARN: "WARN",
		LOG_LEVEL_ERROR: "ERROR",
		LOG_LEVEL_FATAL: "FATAL",

	}



}
//logger运行的主模块，开启多个goroutine写入日志信息
func (logger *Logger)Run() {
	for i,j:=range logger.LogFile{
		defer func() {
			j.Close()
			delete(logger.LogWriter, i)
		}()
	}

	//此处应该改为可配置
	for i:=0;i<10;i++{
		logger.wg.Add(1)
		go logWriter(logger)
	}
	go backupLogFile()


	logger.wg.Wait()

}
//用于写入日志的goroutine
func logWriter(logger *Logger){
	logger.wg.Add(1)
	defer logger.wg.Done()
	for {


		select{
		case msg:=<-logger.LogChan:

			//拼接字符串格式[ALL]\t[time]\t[bus]\t[line]:描述[err]
			buf:=make([]string,13)
			buf[0]="["
			//buf[1]的为logclass，这个放在最后拼接
			buf[2]="]\t["
			buf[3]=msg.Prefix
			buf[4]="]\t["
			buf[5]=msg.Modular
			buf[6]="]\t["
			buf[7]=strconv.Itoa(msg.Line)
			buf[8]="]:"
			buf[9]=msg.Message
			if msg.Err==nil{
				buf[10]="\t"
				buf[11]=""
				buf[12]="\n"
			}else {
				buf[10]="\t["
				buf[11] = msg.Err.Error()
				buf[12]="]\n"
			}
			//将message自带的flag和logger配置中的flag去与，保证输出配置允许的日志
			c:=logger.ConsoleFlag&msg.Flag

			//fmt.Println(c)
			for i,j:=range logger.LogClasss{

				if c&i>0&&i!=1{
					buf[1]=j
					s:=strings.Join(buf,"")
					_,err:=logger.LogWriter[msg.Modular].Write([]byte(s))
					_,err=logger.LogWriter["ALL"].Write([]byte(s))

					//fmt.Println(err)


					if err!=nil{
						fmt.Println("logWriter写入时候出错，message：",msg)
					}
				}
			}
			if msg.Flag&LOG_LEVEL_FATAL>0{
				fmt.Println("程序崩溃！！")
				os.Exit(0)
			}
		default:
			time.Sleep(2*time.Millisecond)
		}

	}
}


func LogIt(des string,err error,flag int){

	//如果err为空，但是log低于8的仍然打印
	//如果log等级大于等于8，则log打印与否取决于err是否为nil
	if err!=nil||flag<8{
		time:=time.Now().Format("2006-01-02 15:04:05.00000")
		_,file,line,_:=runtime.Caller(1)

		fileName:=strings.Split(path.Base(file),".")[0]
		m:=&logMessage{
			Message: des,
			Flag: flag,
			Prefix: time,
			Modular: fileName,
			Line: line,
			Err: err,

		}
		logger.LogChan<-m


	}else{
		return
	}
}

//func main(){
//	logger=NewLogger(ALL)
//	logger.Init()
//	go logger.Run()
//	time.Sleep(2*time.Second)
//	err:=fmt.Errorf("这是一个err")
//	for i:=0;i<10000;i++{
//		LogIt("log"+strconv.Itoa(i),err,INFO)
//		time.Sleep(2*time.Millisecond)
//	}
//	LogIt("log"+strconv.Itoa(65537),err,FATAL)
//
//	select{
//
//	}
//}


func backupLogFile(){
	for{
		for fileName,file:=range logger.LogFile{
			//fmt.Println("test")
			fileInfo,err:=file.Stat()
			if err!=nil{
				continue
			}
			//fmt.Println(fileName,err,fileInfo,logger.ConsoleFlag&1)
			if fileInfo.Size()>=logger.MaxSizeOfLogFile{
				//fmt.Println("test2",fileInfo.Size())
				oldName:=fileInfo.Name()
				t:=time.Now().Format("2006_01_02_15-04-05")
				mutex.Lock()
				file.Close()
				//fmt.Println(oldName)
				err:=os.Rename("../log/"+oldName,"../log/"+fileName+t+".log")
				if err!=nil{
					fmt.Println("备份日志文件失败",err,oldName,"./log/"+fileName+t+".log",t)
				}
				newFile,err:=os.OpenFile("./log/"+oldName,os.O_CREATE|os.O_APPEND,0755)
				logger.LogFile[fileName].Close()

				logger.LogFile[fileName]=newFile
				logger.LogWriter[fileName]=newFile
				mutex.Unlock()


			}
		}
		time.Sleep(10*time.Second)
	}
}
