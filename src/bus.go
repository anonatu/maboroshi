package main

import (
	"bytes"


	//"crypto/dsa"
	"encoding/json"
	"fmt"
	//"io"
	"io/ioutil"
	"net"
	//"os"
	"strconv"
	"time"
)




/*////////////////////////////////报文格式/////////////////////////////////////////
|head(0)|len(1,2)|源节点id(3,4)|源应用号(5,6)|目标节点id(7,8)|目标应用号(9,10)|功能码(11)|参数(12)|本次会话总帧数(13)|本帧序号(14)|DATA(n)|

|<--------------------------------------------------------共15字节--------------------------------------------------> |
	head		:93
	len			:从下一字节开始整个报文的长度，用于长度验证
	源节点		:发信源的节点ID，当本节点ID未确认时应该为0.
	源应用号		:源应用号应该只有1个
							|--0----bus
							|--1----web
	目标节点		:发送目标的节点ID，当为广播时应为65535
	目标应用号	:目标应用可为多个，即应用相互累加即可形成目标应用号
	功能码-参数	:	1----PING
							|--0----加入节点
							|--1----更新节点（此时节点号应为非0固定的节点号）
					2----PONG
							|--0----响应PING加入节点，回复自身的节点号等配置
							|--1----响应PING加入节点，回复自身节点正在busy
							|--2----响应PING更新节点，回复自身节点与发送PING节点号冲突，要求对方重启
					3----DING
							|--0----公布自身节点号，要求其他节点正式存储相关的连接
							|--1----宣布刚刚的PING流程作废，要求各个几点删除缓存的节点数据连接等
					4----DONG
							|--0----宣布本节点正式退出
							|--1----劝退对方节点退出
					5----启动应用
							|--0----启动应用
					6----关闭应用
							|--0----暂停
							|--1----停止
					7----GET   此处报文中data部分应为便于解析的json结构，json中第一个结构应为其特定命令，根据命令的不同，接下来的json结构也不尽相同
								原则上应保证回复的报文可以直接转发给目标节点的目标应用，而不需其他特殊处理
							|--0----请求{"command":""}
							|--1----正常回复

					8----POST
					9----PUT
					101----ERROR
							|--0----busy
							|--1----messageError

	参数			:	1

busyFlag:调用其他节点同功能码，被调用的话则与上128

*///////////////////////////////////////////////////////////////////////////////

//功能码
const(
	BUS_FUNC_CODE_PING=0
	BUS_FUNC_CODE_PONG=1
	BUS_FUNC_CODE_DING=2
	BUS_FUNC_CODE_DONG=3
	BUS_FUNC_CODE_START_APP=4
	BUS_FUNC_CODE_CLOSE_APP=5
	BUS_FUNC_CODE_GET=6
	BUS_FUNC_CODE_POST=7
	BUS_FUNC_CODE_PUT=8
	BUS_FUNC_CODE_DELECT=9
)

//
const(
	COMMAND_CODE_PAUSE=0
	COMMAND_CODE_RESTART=1
	COMMAND_CODE_STOP=2
	COMMAND_CODE_ERROR=255
)
const(
	MODULAR_ID_BUS=0
	MODULAR_ID_WEB=1
	MODULAR_ID_DATABASE=2
	MODULAR_ID_FRONTEND=3


)
const(
	BUS_FLAG_PING		=1<<0
	BUS_FLAG_PINGED_BY	=1<<1
	BUS_FLAG_PONG		=1<<2
	BUS_FLAG_PONGED_BY	=1<<3

)


type Config struct{
	//数据库名称
	DatabaseName string

	//Web
	//是否启动web
	WebStarUp bool
	//web监听的地址，注意格式
	WebServerAddr string


	//Bus
	//Bus节点发现模式
	//0:自动发现
	//1:预配置
	//2:单节点
	NodeDiscoveryMode int
	//udp接收报文的地址
	UDPServerAddr string
	//udp发送广播报文的地址
	UDPClientAddr string
	//tcp服务端地址
	TCPServerAddr string
	TCPClientAddr string
	//预配置的节点号和节点tcpserver地址
	PresetRemoteNodes map[int]string
	//预置本机节点号
	PresetLocalNodeID int
	//节点更新的时间间隔(s)
	BusUpdateCycle time.Duration

}

////////////////////////////////临时定义//////////////////////////////////////

//var config *Config

////////////////////////////////临时定义//////////////////////////////////////

type Node struct{
	//节点ID
	ID 				int
	//运行状态:0 未运行 1 listening 2 accepted 10 busy
	flag			int
	//node节点服务端的ip：端口
	Lstener			net.Listener
	//节点的conn
	Conn 			net.Conn
	//上次节点更新时间，应定时检查，如果超过一定时间没有更新则应断开连接
	LastUpdateTime 	int64
	//如果
	ReceiveChan 	chan []byte
	//用于对外部节点发消息
	SendChan 		chan *message
	//用于将接收到的消息发给Bus，以待进一步分配和反应
	MessageToBus       chan *message
	CmdChanToTcpSender chan byte
	CmdChanToTcpReceiver chan byte
}
//内联函数，产生一个未初始化的node节点
func NewNode(nodeID int,conn net.Conn ,bus *Bus)*Node{

	var node Node
	node.ID=nodeID
	node.Conn=conn
	node.LastUpdateTime=time.Now().Unix()
	node.ReceiveChan=make(chan []byte)
	node.SendChan=make(chan *message)
	//messagetobus应该由bus初始化然后传入
	node.MessageToBus=bus.ToBusChanMap[MODULAR_ID_BUS]
	node.CmdChanToTcpSender =make(chan byte)
	node.CmdChanToTcpReceiver=make(chan byte)

	return &node


}

func initNode(node *Node,bus *Bus)error{
	var retErr error
	if _,ok:=bus.NodeMap[node.ID];ok{
		//如果节点已经存在，检查节点的地址信息是否符合，如果符合则不进行替换
		if bus.NodeMap[node.ID].Conn.RemoteAddr().String()==node.Conn.RemoteAddr().String(){
			retErr=fmt.Errorf("节点号冲突：%d",node.ID)
			return retErr
		}else{
			bus.NodeMap[node.ID].CmdChanToTcpSender <-2
			bus.NodeMap[node.ID].CmdChanToTcpReceiver <-2
			bus.NodeMap[node.ID]=node
			LogIt("节点信息更新",nil,LOG_LEVEL_INFO)
		}
	}else{
		if node.ID>=bus.TempNodeID{
			bus.TempNodeID=node.ID
		}
		bus.NodeMap[node.ID]=node
	}
	return retErr
}

type Bus struct{
	//预配置的节点号

	LocalNodeID int
	//广播conn
	UdpBroadcastConn  *net.UDPConn
	UdpServerConn     *net.UDPConn
	TcpListener       net.Listener
	NodeDiscoveryMode int
	NodeMap           map[int]*Node
	Timer             *TimeFactory
	//
	BusyFlag          int //同功能码
	SendChan          chan int//用于通知bus模块发送广播
	TempNode          *Node
	TempNodeID        int
	//本地节点内部和内外之间通讯使用的通道 int为应用号
	//255：内外之间
	//0：bus
	//1：web
	//2：dataserver
	//3：frontend
	FromBusChanMap map[int]chan *message
	ToBusChanMap   map[int]chan *message
	//总之要发消息就向这里面发
	ToBusSender    chan *message
	TcpAddrString  string


}


type message struct{
	SrcNodeID int
	SrcAppID int
	DesNodeID int
	DesAppID int
	FunctionCode int
	Para int
	Data []byte
}
////对于不规则的json，可先行解析到这种形式，再通过解析command来决定下一步的解析方针
type MessageData struct{
	Command string `json:"command"`
	Para map[string]string	`json:"para"`
	Data interface{} `json:"data"`
}
func newMessageData()MessageData{
	var messageData MessageData
	messageData.Para=make(map[string]string)
	return messageData
}

func (m *message)MakeUp()([][]byte){

	//将message组合为具体报文，如果data部分过长，则应组合为多个帧
	ret:=make([][]byte,1,255)
	n:=len(m.Data)/65000
	for i:=0;i<n+1;i++ {
		var frame bytes.Buffer
		frame.Write([]byte{93})
		//报文长度，暂定为0，用作之后修改
		frame.Write(DisintegrateIntoBytes(0, 2, 0))
		//源节点号，如为加入节点则为0，如为更新节点则从bus中取节点号

		l1, _ := frame.Write(DisintegrateIntoBytes(m.SrcNodeID, 2, 0))
		//源应用号，0：bus
		l2, _ := frame.Write(DisintegrateIntoBytes(m.SrcAppID, 2, 0))
		//目标节点id
		l3,_ := frame.Write(DisintegrateIntoBytes(m.DesNodeID, 2, 0))
		l4, _ := frame.Write(DisintegrateIntoBytes(m.DesAppID, 2, 0))
		//功能码
		l5, _ := frame.Write([]byte{byte(m.FunctionCode)})
		//功能参数
		l6, _ := frame.Write([]byte{byte(m.Para)})

		//本次会话帧数
		l7, _ := frame.Write([]byte{byte(n+1)})

		//帧序号
		l8, _ := frame.Write([]byte{byte(i)})
		var l9 int
		if i==n+i {
			l9, _ = frame.Write(m.Data[i*6500 : len(m.Data)])
		}else{
			l9, _ = frame.Write(m.Data[i*6500 : (i+1)*65000])
		}
		frameBytes:=frame.Bytes()
		tmp:=DisintegrateIntoBytes(l1+l2+l3+l4+l5+l6+l7+l8+l9,2,0)
		frameBytes[1]=tmp[0]
		frameBytes[2]=tmp[1]

		ret[i]=frameBytes
	}


	return ret
}



func (bus *Bus)InitConfig(config *Config){

	LogIt("BUS配置正在初始化...",nil,LOG_LEVEL_INFO)
	//两种模式分别对应自动发现模式和预置模式
	//如果是预置模式，则要求在这里能初始化的的属性全部在这里初始化完成，在RUN方法中可以直接使用

	bus.LocalNodeID=0
	jsonBytes,err:=ioutil.ReadFile("./config.json")
	LogIt("读取config.json失败",err,LOG_LEVEL_FATAL)
	err=json.Unmarshal(jsonBytes,config)
	LogIt("解析config.json失败",err,LOG_LEVEL_FATAL)
	switch config.NodeDiscoveryMode{
	case 0:
		bus.LocalNodeID=0
	case 1:
		bus.LocalNodeID=config.PresetLocalNodeID
	default:
		LogIt("节点发现机制配置错误",err,LOG_LEVEL_FATAL)

	}

	bus.NodeMap=make(map[int]*Node)
	//bus.NodeMap[0]=NewNode(0,nil,bus)
	bus.Timer= NewFactory()
	bus.BusyFlag=0
	bus.SendChan=make(chan int,3)
	bus.FromBusChanMap =make(map[int]chan *message,64)
	bus.ToBusChanMap =make(map[int]chan *message,64)
	//不同模块之间的通讯通道
	bus.FromBusChanMap[0]=make(chan *message,64)
	bus.FromBusChanMap[1]=make(chan *message,64)
	bus.FromBusChanMap[2]=make(chan *message,64)
	bus.FromBusChanMap[3]=make(chan *message,64)
	//外部节点发来的信息就送进这个通道
	bus.FromBusChanMap[255]=make(chan *message,64)
	//不同模块之间的通讯通道
	bus.ToBusChanMap[0]=make(chan *message,64)
	bus.ToBusChanMap[1]=make(chan *message,64)
	bus.ToBusChanMap[2]=make(chan *message,64)
	bus.ToBusChanMap[3]=make(chan *message,64)
	//外部节点发来的信息就送进这个通道
	bus.ToBusChanMap[255]=make(chan *message,64)

	//检查地址是否正确
	us,err:=net.ResolveUDPAddr("udp",config.UDPServerAddr)
	LogIt("UDPServer地址格式解析失败",err,LOG_LEVEL_ERROR)
	uc,err:=net.ResolveUDPAddr("udp",config.UDPClientAddr)
	LogIt("UDPClinet地址格式解析失败",err,LOG_LEVEL_ERROR)
	ip:=us.IP.To4()
	urc:=&net.UDPAddr{
		IP:[]byte{ip[0],ip[1],ip[2],255},
		Port: us.Port,
		Zone: "",
	}




	bus.UdpServerConn,err=net.ListenUDP("udp",us)
	LogIt("UPDServer连接失败",err,LOG_LEVEL_ERROR)

	bus.UdpBroadcastConn,err = net.DialUDP("udp",uc,urc)
	LogIt("UPD广播连接失败",err,LOG_LEVEL_ERROR)
	bus.ToBusSender =make(chan *message,512)
	bus.TcpListener,err=net.Listen("tcp",config.TCPServerAddr)
	LogIt("创建TCPListenner失败",err,LOG_LEVEL_ERROR)
	bus.TcpAddrString=config.TCPServerAddr
	LogIt("BUS配置初始化完毕",nil,LOG_LEVEL_INFO)

}

func SendMessage(conn net.Conn,message [][]byte)error{

	var err error
	if len(message)==0{
		return fmt.Errorf("air frame")
	}
	switch len(message){
	case 0:
		err=fmt.Errorf("air frame")
		return err
	case 1:
		_,err=conn.Write(message[0])
		LogIt("发送消息失败",err,LOG_LEVEL_ERROR)
		if err!=nil{
			return err
		}
		return nil
	default:
		for _,f:=range message{
			_,err=conn.Write(f)
			LogIt("发送消息失败",err,LOG_LEVEL_ERROR)
			if err!=nil{
				return err
			}
			return nil
		}

	}
	return nil
}
func (bus *Bus)DeleteNode(nodeID int){
	bus.NodeMap[nodeID].Conn.Close()
	bus.NodeMap[nodeID].CmdChanToTcpSender <-2
	bus.NodeMap[nodeID].CmdChanToTcpReceiver<-2
	delete(bus.NodeMap,nodeID)
}








//	for{
//		var data []byte
//		var c int
//		select{
//		case data=<-bus.LocalChanMap[0]:
//		case c =<-bus.SendChan:
//		}
//	}
//}

//PING流程
//通过广播发送自己tcpserver的地址
//运行状态后定期发送
func (bus *Bus)PING()error{
	LogIt("PING",nil,LOG_LEVEL_INFO)
	bus.BusyFlag=0
	var para int

	if bus.LocalNodeID!=0{
		para=1
	}


	PINGMessage:=message{bus.LocalNodeID,0,65535,0,1,para,[]byte{}}
	if bus.LocalNodeID>0{
		PINGMessage.Para=1
	}
	tcpTddr,_:=net.ResolveTCPAddr("tcp",bus.TcpAddrString)
	PINGMessage.Data=append(tcpTddr.IP.To4(),DisintegrateIntoBytes(tcpTddr.Port,2,0)...)
	message:=PINGMessage.MakeUp()

	err:=SendMessage(bus.UdpBroadcastConn,message)
	LogIt("PING发送失败",err,LOG_LEVEL_ERROR)
	if err!=nil{
		return err
	}
	if bus.LocalNodeID==0 {
		bus.BusyFlag |= BUS_FLAG_PING
	}else{

		bus.BusyFlag=0
	}
	return nil

}
func (bus *Bus)PINGedBy(msg *message)error{
	LogIt("PINGed",nil,LOG_LEVEL_INFO)
	var retErr error
	if len(msg.Data)!=6{
		retErr=fmt.Errorf("地址解析失败")
		LogIt("收到的PING报文data部分长度不正确",nil,LOG_LEVEL_INFO)
		return nil
	}
	if len(bus.NodeMap)==0{
		bus.LocalNodeID=1
	}

	addr:=net.TCPAddr{
		IP: msg.Data[0:4],
		Port:MakeUpInt(msg.Data[4:6],0),
	}
	if msg.Para==1{
		//更新的情况
		if addr.String()==bus.NodeMap[msg.SrcNodeID].Conn.LocalAddr().String(){
			return nil
		}
		//节点号和IP对不上
		LogIt("PINGed节点号和ID差错",nil,LOG_LEVEL_ERROR)

	}
	//表记flag
	bus.BusyFlag|=BUS_FLAG_PING
	conn,err:=net.Dial("tcp",addr.String())
	if err!=nil{
		retErr=fmt.Errorf("BUS连接TCP服务端失败")
		LogIt("BUS连接TCP服务端失败",err,LOG_LEVEL_WARN)
		return retErr
	}

	LogIt("连接到tcp服务端",err,LOG_LEVEL_INFO)
	str:=fmt.Sprintf("已连接服务端，远端地址为%s",conn.RemoteAddr())
	LogIt(str,nil,LOG_LEVEL_INFO)
	bus.TempNode=NewNode(0,conn,bus)
	go TcpReceiver(bus.TempNode,nil)
	go TcpSender(bus.TempNode)
	err=bus.PONG(conn)
	if err!=nil{
		retErr=fmt.Errorf("发送PONG失败")
		LogIt("发送PONG失败",err,LOG_LEVEL_ERROR)
		return retErr
	}
	return retErr
}

func (bus *Bus)PONG(conn net.Conn)error{
	LogIt("PONG",nil,LOG_LEVEL_INFO)
	var m message
	m.SrcNodeID=bus.LocalNodeID
	m.SrcAppID=0
	m.DesNodeID=65535
	m.DesAppID=0
	m.FunctionCode=2

	m.Data=[]byte{}

	//switch bus.BusyFlag{
	////已经被ping
	//case :
	//
	m.Para=0

	err:=SendMessage(conn,m.MakeUp())

	if err!=nil{
		err=fmt.Errorf("can not send PONG.")
		LogIt("PONG发送失败",err,LOG_LEVEL_ERROR)
		return err
	}
	bus.BusyFlag|=BUS_FLAG_PONG
	return nil
	////已经由由我ping
	//case 1:
	//	bus.BusyFlag=0
	//	m.Para=1
	//	err:=SendMessage(bus.UdpBroadcastConn,m.MakeUp())
	//	LogIt("PONG发送失败",err,LOG_LEVEL_ERROR)
	//
	//
	//	err = fmt.Errorf("this node had PINGed,can not PONG now.")
	//
	//	return err
	//
	//default:
	//	err:=fmt.Errorf("bus.busyFlag Error,can not PONG.")
	//	return err
	//}


}


func (bus *Bus)DING()error{
	LogIt("DING",nil,LOG_LEVEL_INFO)
	bus.LocalNodeID=bus.TempNodeID+1
	var m message
	m.SrcNodeID=bus.LocalNodeID
	m.SrcAppID=0
	m.DesNodeID=65535
	m.DesAppID=0
	m.FunctionCode=3
	//正在主动PONG
	m.Para=0
	m.Data,_=AddrToBytes(bus.TcpAddrString,"tcp")


	err:=SendMessage(bus.UdpBroadcastConn,m.MakeUp())
	if err!=err{
		return err
	}
	bus.BusyFlag=0
	return nil
}
func (bus *Bus)DONG(nodeID int)error{
	LogIt("DONG",nil,LOG_LEVEL_INFO)
	var m message
	m.SrcNodeID=bus.LocalNodeID
	m.SrcAppID=0
	m.DesNodeID=65535
	m.DesAppID=0
	m.FunctionCode=4
	if nodeID==65535{
		m.Para=0
		m.Data=[]byte{}
		err:=SendMessage(bus.UdpBroadcastConn,m.MakeUp())
		if err!=err{
			return err
		}
		for _,node:=range bus.NodeMap{
			node.CmdChanToTcpSender <-2
			node.CmdChanToTcpReceiver <-2
		}
		bus.NodeMap=make(map[int]*Node)
	}else{
		m.Para=1
		m.Data=[]byte{}
		err:=SendMessage(bus.NodeMap[nodeID].Conn,m.MakeUp())
		if err!=err{
			return err
		}
	}


	bus.BusyFlag=0
	return nil
}
func (bus *Bus)DONGedBy(msg *message)error{
	var retErr error
	switch msg.Para{
	case 0:
		bus.NodeMap[msg.SrcNodeID].CmdChanToTcpSender <-2
		bus.NodeMap[msg.SrcNodeID].CmdChanToTcpReceiver<-2
		bus.NodeMap[msg.SrcNodeID].Conn.Close()
		delete(bus.NodeMap,msg.SrcNodeID)
	case 1:
		bus.DONG(65535)
	default:
		LogIt("PONGed：para错误",nil,LOG_LEVEL_INFO)

	}
	return retErr
}


func MakeUpInt(bytes []byte ,order int)int{
	//将byte切片合成为int，order为0时，高字节在前，order为1时，低字节在前
	var ret int
	switch order{

	case 0:
		//顺序
		for _,j:=range bytes{
			ret=ret<<8+int(j)
		}
		return ret
	case 1:
		//逆序
		for i,j:=range bytes{
			ret=ret+int(j)<<(8*i)
		}
		return ret
	default:
		return 0

	}

}
func DisintegrateIntoBytes(number int,byteNumber int,order int)[]byte{
	//将int解散为[]Byte

	ret:=make([]byte,byteNumber)
	switch order{
	case 0:
		for i:=0;i<byteNumber;i++{
			ret[byteNumber-i-1]=byte((number>>(8*i))&255)
		}
		return ret
	case 1:
		for i:=0;i<byteNumber;i++{
			ret[i]=byte(number&(255<<(8*i)))
		}
		return ret
	default:
		return ret

	}

}
//将[][]byte报文转化为message
func BytesToMessage(bs [][]byte)(*message){
	//1.组合报文中的data
	var msg message
	//var errRet error
	var data bytes.Buffer
	for _,b:=range bs{
		data.Write(b[15:])
	}

	//2.解析为message
	temp:=bs[0]
	msg.SrcNodeID=MakeUpInt(temp[3:5],0)
	msg.SrcAppID=MakeUpInt(temp[5:7],0)
	msg.DesNodeID=MakeUpInt(temp[7:9],0)
	msg.DesAppID=MakeUpInt(temp[9:11],0)
	msg.FunctionCode=int(temp[11])
	msg.Para=int(temp[12])
	msg.Data=data.Bytes()
	return &msg
}
//将message转化为[][]byte报文
func MessageToBytes(m *message)([][]byte,error){
	var retErr error
	ret:=make([][]byte,0)
	dataLen:=len(m.Data)

	var tmpBuffer bytes.Buffer
	_,err:=tmpBuffer.Write([]byte{92,0,0})
	if err!=nil{
		retErr=fmt.Errorf("MessageToBytes解析失败")
		return nil,retErr
	}
	_,err=tmpBuffer.Write(DisintegrateIntoBytes(m.SrcNodeID,2,0))
	if err!=nil{
		retErr=fmt.Errorf("MessageToBytes:SrcNodeID解析失败")
		return nil,retErr
	}
	_,err=tmpBuffer.Write(DisintegrateIntoBytes(m.SrcAppID,2,0))
	if err!=nil{
		retErr=fmt.Errorf("MessageToBytes:SrcAppID解析失败")
		return nil,retErr
	}
	_,err=tmpBuffer.Write(DisintegrateIntoBytes(m.DesNodeID,2,0))
	if err!=nil{
		retErr=fmt.Errorf("MessageToBytes:DesNodeID解析失败")
		return nil,retErr
	}
	_,err=tmpBuffer.Write(DisintegrateIntoBytes(m.DesAppID,2,0))
	if err!=nil{
		retErr=fmt.Errorf("MessageToBytes:DesAppID解析失败")
		return nil,retErr
	}
	_,err=tmpBuffer.Write([]byte{byte(m.FunctionCode),byte(m.Para),byte((len(m.Data)/65000)+1)})
	if err!=nil{
		retErr=fmt.Errorf("MessageToBytes:funcCode,Para,data解析失败")
		return nil,retErr
	}
	for i:=0;i<=(len(m.Data)/65000);i++{

		_,err=tmpBuffer.Write([]byte{byte(i)})
		var t int
		var tmpLen int
		if t=dataLen-i*65000;t<65000{
			tmpLen, err = tmpBuffer.Write(m.Data[i*65000:t])
			if err!=nil{
				retErr=fmt.Errorf("MessageToBytes:data首帧解析失败")
				return nil,retErr
			}
		}else {
			tmpLen, err = tmpBuffer.Write(m.Data[i*65000:(i+1)*65000])
			if err!=nil{
				retErr=fmt.Errorf("MessageToBytes:data非首帧解析失败")
				return nil,retErr
			}
		}
		tmpSlice:=tmpBuffer.Bytes()
		tmp:=DisintegrateIntoBytes(tmpLen+12,2,0)
		tmpSlice[1]=tmp[0]
		tmpSlice[2]=tmp[1]
		ret=append(ret,tmpSlice)
	}

	LogIt("拼装报文失败",err,LOG_LEVEL_ERROR)

	return ret,nil

}
func (bus *Bus)TcpAccepter(){
	LogIt("TcpAccepter启动",nil,LOG_LEVEL_INFO)
	for {
		conn,err:=bus.TcpListener.Accept()
		if err!=nil{
			str:=fmt.Sprintf("TcpAccept失败，addr：%s",conn.RemoteAddr().String())
			LogIt(str,err,LOG_LEVEL_WARN)
			continue
		}
		//链接后三秒钟内应该发送Pong报文，否则将断开连接
		tmpNode:=NewNode(0,conn,&BUS)
		ret:=make(chan int,1)
		go TcpSender(tmpNode)
		go TcpReceiver(tmpNode,ret)
		timer:=time.NewTimer(3*time.Second)
		select {
		case ID:=<-ret:
			//这里确定好了conn和节点号之间的关系。通过init将节点存入bus
			tmpNode.ID=ID
			err:=initNode(tmpNode,&BUS)

			if err!=nil{
				LogIt("initNode失败",err,LOG_LEVEL_ERROR)
				tmpNode.CmdChanToTcpSender <-2
				tmpNode.CmdChanToTcpReceiver<-2
				conn.Close()
				continue
			}
			//if bus.TempNodeID<=ID{
			//	bus.TempNodeID=ID
			//}

		case <-timer.C:
			tmpNode.CmdChanToTcpSender <-2
			tmpNode.CmdChanToTcpReceiver<-2
			conn.Close()
			continue
		}
	}
}
func (bus *Bus)UdpAccepter(){
	LogIt("UdpAccepter启动...",nil,LOG_LEVEL_INFO)
	tmpMsg:=make([][]byte,0)
	for {
		buf:=make([]byte,65535)
		n,addr,err:=bus.UdpServerConn.ReadFromUDP(buf)

		if addr.String()==Cfg.UDPClientAddr{
			//屏蔽本机的广播
			//ln("pingbi",bus.UdpBroadcastConn.LocalAddr().String(),bus.UdpBroadcastConn.RemoteAddr().String())
			continue
		}

		if err!=nil{
			LogIt("udp接收消息异常",err,LOG_LEVEL_WARN)
			continue
		}
		bs:=buf[0:n]
		if len(bs)-3 != MakeUpInt(bs[1:3], 0) || len(bs) < 15 {
			continue
		}
		if bs[14] == 0 {
			tmpMsg = make([][]byte, int(bs[13]))
		}

		if bs[14] == bs[13]-1 {
			tmpMsg[int(bs[13])-1] = bs
			message := BytesToMessage(tmpMsg)

			bus.ToBusChanMap[MODULAR_ID_BUS] <- message
		} else {
			tmpMsg[int(bs[13])-1] = bs
		}



	}
}
func TcpSender(node *Node) {
	LogIt("TcpSender启动",nil,LOG_LEVEL_INFO)
	useFlag:=1
	for{
		if useFlag==0{
			time.Sleep(1*time.Second)
			continue
		}
		select {
		//收到命令 0:暂停通道发送,1:重新开始发送,2:停止
		case cmd := <-node.CmdChanToTcpSender:
			switch cmd {
			case COMMAND_CODE_PAUSE:
				useFlag=0
				LogIt("通道暂停使用.通道号:"+strconv.Itoa(node.ID),nil,LOG_LEVEL_INFO)
			case COMMAND_CODE_RESTART:
				useFlag=1
				LogIt("通道重新开始使用.通道号:"+strconv.Itoa(node.ID),nil,LOG_LEVEL_INFO)
			case COMMAND_CODE_STOP:
				LogIt("通道停止使用.通道号:"+strconv.Itoa(node.ID),nil,LOG_LEVEL_INFO)
				return
			}
		case msg := <-node.SendChan:
			bytesList,err:=MessageToBytes(msg)
			if err!=nil {
				LogIt("message解析失败", err, LOG_LEVEL_ERROR)
			}
			for _,bs:=range bytesList{
				_,err:=node.Conn.Write(bs)
				if err!=nil {
					LogIt("message发送", err, LOG_LEVEL_ERROR)
					break
				}
			}
		}
	}
}
func TcpReceiver(node *Node,ret chan int){
	LogIt("TcpReceiver启动",nil,LOG_LEVEL_INFO)
	var tmpMsg [][]byte
	for{
		tmpBs:=make([]byte,65535)
		n,err:=node.Conn.Read(tmpBs)
		if err != nil {
			LogIt("tcpWorker接收失败,node号:"+strconv.Itoa(node.ID), err, LOG_LEVEL_ERROR)
			return
		}
		bs:=tmpBs[0:n]
		select {

		case <-node.CmdChanToTcpReceiver:
			return
		default:
			if len(bs)-3 != MakeUpInt(bs[1:3], 0) || len(bs) < 15 {
				continue
			}
			if bs[14] == 0 {
				tmpMsg = make([][]byte, int(bs[13]))
			}

			if bs[14] == bs[13]-1 {
				tmpMsg[int(bs[13])-1] = bs
				message := BytesToMessage(tmpMsg)
				if message.FunctionCode==2&&(BUS.BusyFlag&BUS_FLAG_PING>0){
					LogIt("PONGed",err,LOG_LEVEL_INFO)
					ret<-message.SrcNodeID
					continue
				}
				node.MessageToBus <- message
			} else {

				tmpMsg[int(bs[13])-1] = bs
			}
		}

	}
}


func BusCoreProcessor(bus *Bus){
	LogIt("BusCoreProcessor启动",nil,LOG_LEVEL_INFO)
	//计时器，在收到ping之后开始计时，如果实现内没能完成pingpongdong流程，则初始化关键的变量
	timer:=make([]int64,4)
	for{
		PINGChan,err:=Timer.GetChannel("PING")
		if err!=nil{
			time.Sleep(20*time.Millisecond)
			continue
		}

		select{

		case <-PINGChan:
			//超时的情况
			if len(bus.NodeMap)==0{

			}else {
				bus.BusyFlag = 0
				bus.LocalNodeID = bus.TempNodeID + 1

				_ = bus.DING()
			}

		case msg,ok:=<-bus.ToBusChanMap[MODULAR_ID_BUS]:

			if !ok{
				continue
			}
			switch msg.FunctionCode{
			case 1:
				//收到PING
				err:=bus.PINGedBy(msg)
				if err!=nil{
					bus.BusyFlag=0
				}
				timer[1]=time.Now().Unix()

			//case 2:
			//	//收到PONG
			//	LogIt("PONGed",nil,LOG_LEVEL_INFO)
			//	if bus.BusyFlag&BUS_FLAG_PING>0{
			//		continue
			//	}
			//
			//	//未超时
			//	if msg.SrcAppID>bus.TempNodeID{
			//		bus.TempNodeID=msg.SrcAppID
			//	}
			//	bus.BusyFlag=0
			//	bus.BusyFlag=BUS_FLAG_PONGED_BY
			case 3:
				//收到DING
				LogIt("DINGed",nil,LOG_LEVEL_INFO)
				if bus.BusyFlag&BUS_FLAG_PONG>0 {
					bus.TempNode.ID=msg.SrcNodeID
					initNode(bus.TempNode,bus)
					//bus.NodeMap[msg.SrcNodeID]=bus.TempNode
				}else{
					continue
				}
				bus.BusyFlag=0
				bus.TempNode=nil
			case 4:
				//收到DONG
				bus.DONGedBy(msg)
			case 7:
				//接收到get指令的回复
				//GET
				md,err:=msg.GetMessageData()
				if err!=nil{
					LogIt("messageData解析失败",nil,LOG_LEVEL_ERROR)
				}
				switch msg.Para {

				case 0:
					//收到外部节点的Get请求


					replyData, err := HandleGetRequestMsgData(md)
					if err != nil {
						LogIt("bus message处理失败", err, LOG_LEVEL_ERROR)
						continue
					}

					err = ReplyMessage(msg, replyData, bus.ToBusSender)
					if err != nil {
						LogIt("message回复失败", err, LOG_LEVEL_ERROR)
						continue
					}


				case 1:
					//收到外部节点的Get请求回复

					bus.FromBusChanMap[MODULAR_ID_WEB]<-msg
					//err = ReplyMessage(msg, replyData, bus.FromBusChanMap[MODULAR_ID_WEB])
					//if err != nil {
					//	LogIt("message回复失败", err, LOG_LEVEL_ERROR)
					//	continue
					//}

				}



				}
		case msg,ok:=<-bus.ToBusChanMap[MODULAR_ID_WEB]:
			if !ok{
				continue
			}
			switch msg.FunctionCode{

			case 7:
				//GET
				md,err:=msg.GetMessageData()
				if err!=nil{
					LogIt("messageData解析失败",nil,LOG_LEVEL_ERROR)
				}
				switch msg.Para{

				case 0:
					//请求
					DesNodeIDStr,ok:=md.Para["Node_ID"]
					if ok {
						DesNodeID, _ := strconv.Atoi(DesNodeIDStr)
						msg.DesNodeID = DesNodeID

						if DesNodeID==bus.LocalNodeID{
							replyData,err:=HandleGetRequestMsgData(md)
							if err!=nil{
								LogIt("bus message处理失败",err,LOG_LEVEL_ERROR)
								continue
							}
							err=ReplyMessage(msg,replyData,bus.ToBusSender)

							if err!=nil{
								LogIt("message回复失败",err,LOG_LEVEL_ERROR)
								continue
							}
						}else{
							bus.NodeMap[msg.DesNodeID].SendChan<-msg


						}
					}else{
						replyData,err:=HandleGetRequestMsgData(md)
						if err!=nil{
							LogIt("bus message处理失败",err,LOG_LEVEL_ERROR)
							continue
						}
						err=ReplyMessage(msg,replyData,bus.ToBusSender)
						if err!=nil{
							LogIt("message回复失败",err,LOG_LEVEL_ERROR)
							continue
						}
					}


				default:

				}

			case 8:
			default:
			}

		case msg,ok:=<-bus.ToBusChanMap[MODULAR_ID_DATABASE]:
			if !ok{
				continue
			}
			fmt.Println(msg)


		case msg,ok:=<-bus.ToBusChanMap[MODULAR_ID_FRONTEND]:
			if !ok{
				continue
			}
			fmt.Println(msg)

		case msg,ok:=<-bus.ToBusSender:

			if !ok{
				continue
			}

			if msg.DesNodeID==bus.LocalNodeID{
				//目标是本机某个模块的情况
				bus.FromBusChanMap[msg.DesAppID]<-msg
			}else{
				//收到的是广播信息的情况
				if msg.DesNodeID==65535{
					bus.ToBusChanMap[msg.DesAppID]<-msg
				}
				//目标是外部节点的情况
				messageBytes,err:=MessageToBytes(msg)
				if err!=nil{
					continue
				}
				for _,m:=range messageBytes {

					_,err=bus.NodeMap[msg.DesNodeID].Conn.Write(m)
					if err!=nil{
						break
					}
				}
			}
			//发送
		//default:
		//	continue
		}
	}


//////////////////////////////////////////////////////////////////////////
	//for {
	//	msg := <-bus.ToBusSender
	//	if msg.DesNodeID != bus.LocalNodeID && msg.DesNodeID == 65535 {
	//		continue
	//	}
	//
	//	switch msg.DesAppID {
	//	//bus
	//	case 0:
	//
	//		switch msg.FunctionCode {
	//		//PING
	//		case 1:
	//
	//			if msg.Para==0{
	//				bus.BusyFlag=128+1
	//				_=bus.PONG()
	//				bus.BusyFlag=128+1
	//			}else{
	//				bus.NodeMap[msg.SrcNodeID].LastUpdateTime=time.Now().Unix()
	//			}
	//
	//
	//		case 2:
	//			if bus.TempNodeID<=msg.SrcNodeID{
	//				bus.TempNodeID+=1
	//				bus.BusyFlag=128+2
	//				continue
	//			}
	//			continue
	//		case 3:
	//			//收到Ding
	//			if bus.BusyFlag!=128+2{
	//				continue
	//			}
	//			var tmpNode Node
	//			tmpNode.ID=msg.SrcNodeID
	//			tmpNode.LastUpdateTime=time.Now().Unix()
	//			addr,err:=BytesToAddr(msg.Data)
	//			if err!=nil{
	//				continue
	//			}
	//			tmpNode.Conn,err=net.Dial("tcp",addr)
	//			if err!=nil{
	//				continue
	//			}
	//			tmpNode.ReceiveChan=make(chan []byte,512)
	//			tmpNode.SendChan=make(chan *message,512)
	//			bus.NodeMap[tmpNode.ID]=&tmpNode
	//
	//		case 4:
	//		case 5:
	//		case 6:
	//		case 7:
	//			//Get
	//
	//			switch msg.Para{
	//
	//			case 0:
	//				//请求
	//				replyData,err:=HandleGetRequestMsgData(msg)
	//				if err!=nil{
	//					LogIt("bus message处理失败",err,LOG_LEVEL_ERROR)
	//					continue
	//				}
	//				err=ReplyMessage(msg,replyData,bus.NodeMap[msg.SrcNodeID].SendChan)
	//				if err!=nil{
	//					LogIt("message回复失败",err,LOG_LEVEL_ERROR)
	//					continue
	//				}
	//			case 1:
	//				//回复请求
	//			default:
	//
	//			}
	//
	//		case 8:
	//		case 9:
	//		case 101:
	//		default:
	//
	//		}
	//	case 1:
	//	default:
	//
	//	}
	//}

}
func BytesToAddr(bytes []byte)(string,error){
	var retErr error
	if len(bytes)!=6{
		retErr=fmt.Errorf("the []byte can not make up Addr")
		return "",retErr
	}
	var IP net.IP
	IP=bytes[0:4]
	ret:=IP.String()+":"+strconv.Itoa(MakeUpInt(bytes[4:],0))
	return ret,nil
}
func AddrToBytes(addr string,network string)([]byte,error){
	var ret []byte
	var retErr error
	switch network {
	case "TCP":
		tmp,err:=net.ResolveTCPAddr("tcp",addr)
		if err!=nil{
			retErr=fmt.Errorf("TCP地址解析失败")
			return nil,retErr
		}
		ret=append(tmp.IP,DisintegrateIntoBytes(tmp.Port,2,0)...)
		return ret,nil
	case "UDP":
		tmp,err:=net.ResolveUDPAddr("udp",addr)
		if err!=nil{
			retErr=fmt.Errorf("UDP地址解析失败")
			return nil,retErr
		}
		ret=append(tmp.IP,DisintegrateIntoBytes(tmp.Port,2,0)...)
		return ret,nil
	default:
		retErr=fmt.Errorf("未知network类型")
		return nil,retErr
	}

}



func RunBus(bus *Bus){
	//timer.Hire(1)

	//初始化bus
	//bus.InitConfig(&Cfg)
	//启动处理器
	go BusCoreProcessor(bus)
	//sent PING
	go bus.TcpAccepter()
	go bus.UdpAccepter()
	time.Sleep(1*time.Second)

	updateCycle:=time.Second*Cfg.BusUpdateCycle
	ticker:=time.Tick(updateCycle)
	bus.PING()
	_,err:=Timer.Hire("PING",1000*5,1,1)
	if err!=nil{
		LogIt("PING计时器开启失败",err,LOG_LEVEL_WARN)
	}
	for{
		select{
		case <-ticker:
			bus.PING()
			_,_=Timer.Hire("PING",1000*5,1,1)
		}

	}
}
//func JoinNodes(bus *Bus){
//	switch bus.NodeDiscoveryMode {
//
//	case 0:
//		for {
//			err := bus.PING()
//			time.Sleep(3 * time.Second)
//			if err != nil || bus.BusyFlag == 0 {
//				time.Sleep(10 * time.Second)
//				continue
//			}
//			time.Sleep(30 * time.Second)
//			err = bus.DING()
//			if err != nil || bus.BusyFlag == 2 {
//				time.Sleep(10 * time.Second)
//				continue
//			}
//			time.Sleep(5 * time.Second)
//			break
//		}
//	case 1:
//	default:
//	}
//}
func (msg *message)GetMessageData()(*MessageData,error){
	ret :=newMessageData()

	var retErr error
	var tmpMap map[string]interface{}
	err:=json.Unmarshal(msg.Data,&tmpMap)
	if err!=nil{
		LogIt("解析web发来的json失败",err,LOG_LEVEL_ERROR)
		retErr=fmt.Errorf("json解析失败")

		return nil,retErr
	}

	ret.Command=tmpMap["command"].(string)
	tmpPara:=tmpMap["para"].(map[string]interface{})
	for key,value:=range tmpPara{
		switch value.(type){
		case string:
			ret.Para[key]=value.(string)
		case int:
			ret.Para[key]=strconv.Itoa(value.(int))
		case float64:
			ret.Para[key]=strconv.FormatFloat(value.(float64), 'f', 0, 64)
		default:
			retErr=fmt.Errorf("解析json中para类型出错")
			return nil, retErr
		}

	}
	return &ret,nil
}
//解析收到的messagedata，执行命令并返回一个用于回复的[]byte
func HandleGetRequestMsgData(md *MessageData)([]byte,error){

	var retErr error
	var err error
	var tmpMap map[string]interface{}
	switch md.Command{
	case "getPointsFromDB":
		//读取点配置信息
		pointType,err:=strconv.Atoi(md.Para["Point_Type"])
		if err!=nil{
			retErr=fmt.Errorf("messagedata解析失败")
			LogIt("getPointsFromDB:PointType解析失败",err,LOG_LEVEL_ERROR)
			return nil,retErr
		}
		channelID,err:=strconv.Atoi(md.Para["Channel_ID"])
		if err!=nil{
			retErr=fmt.Errorf("messagedata解析失败")
			LogIt("getPointsFromDB:channelID解析失败",err,LOG_LEVEL_ERROR)
			return nil,retErr
		}
		rtuID,err:=strconv.Atoi(md.Para["Rtu_ID"])
		if err!=nil{
			retErr=fmt.Errorf("getPointsFromDB:rtuID解析失败",err,LOG_LEVEL_ERROR)
			return nil,retErr
		}
		md.Data,err= getPointsFromDB(pointType,channelID,rtuID)
		if err!=nil{
			retErr=fmt.Errorf("getPointsFromDB:Data解析失败",err,LOG_LEVEL_ERROR)
			return nil,retErr
		}
	case "getChannelsFromDB":
		//读取指定数据库的channel列表、需传入数据库名
		nodeID,err:=strconv.Atoi(md.Para["Node_ID"])
		if err!=nil{
			retErr=fmt.Errorf("getChannelsFromDB:nodeID解析失败")
			return nil, retErr
		}
		if nodeID==BUS.LocalNodeID{
			//当目标节点是本机时
			md.Data,err=getChannelsFromDB()
			if err!=nil {
				retErr = fmt.Errorf("getChannelsFromDB:Data解析失败", err, LOG_LEVEL_ERROR)
				return nil, retErr
			}
		}else{
			//当目标节点不是本机时



		}
	case "getRtusFromDB":
		nodeID,err:=strconv.Atoi(md.Para["Node_ID"])
		if err!=nil{
			retErr=fmt.Errorf("getRtusFromDB:nodeID解析失败")
			return nil, retErr
		}
		channelID,err:=strconv.Atoi(md.Para["Channel_ID"])
		if err!=nil{
			retErr=fmt.Errorf("getRtusFromDB:channelID解析失败")
			return nil, retErr
		}
		md.Data,err=getRtusFromDB(nodeID,channelID)
		if err!=nil {
			retErr = fmt.Errorf("getRtusFromDB:Data解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
	case "getDatabaseFromDB":
		//读取指定的节点的数据库列表，在调用这个函数时候已经确定目标节点的ID，故无需传入这个参数
		md.Data,err= getDatabaseList()
		if err!=nil{
			retErr = fmt.Errorf("getDatabaseFromDB:Data解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
	case "getNodesFromDB":
		//读取本机存储的节点信息
		md.Data,err=getNodeList()
		if err!=nil{
			retErr = fmt.Errorf("getNodesFromDB:Data解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
	case "getPointsFromRTD"	:{
		pointType,err:=strconv.Atoi(md.Para["Point_Type"])
		if err!=nil{
			retErr = fmt.Errorf("getPointsFromRTD:pointType解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
		channelID,err:=strconv.Atoi(md.Para["Channel_ID"])
		if err!=nil{
			retErr = fmt.Errorf("getPointsFromRTD:channelID解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
		rtuID,err:=strconv.Atoi(md.Para["Rtu_ID"])
		if err!=nil{
			retErr = fmt.Errorf("getPointsFromRTD:rtuID解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
		md.Data,err=getPointsFromRTD(channelID,rtuID,pointType)
		if err!=nil{
			retErr = fmt.Errorf("getPointsFromRTD:Data解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
	}
	case "getPointType":
		md.Data,err=getPointType()
		if err!=nil{
			retErr = fmt.Errorf("getPointType:Data解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
	case "setPointToDB":
		flag,err:=strconv.Atoi(md.Para["Flag"])
		if err!=nil{
			retErr = fmt.Errorf("setPoint:flag解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}

		err=setPointToDB(tmpMap["data"],flag)
		if err!=nil{
			retErr = fmt.Errorf("setPoint:data解析失败", err, LOG_LEVEL_ERROR)
			md.Para["Flag"]=strconv.Itoa(flag+128)
		}

	case "deletePointToDB":
		pointType,err:=strconv.Atoi(md.Para["Point_Type"])
		err=deletePointToDB(tmpMap["data"],pointType)
		if err!=nil{
			retErr = fmt.Errorf("deletePointToDB:PointType解析失败", err, LOG_LEVEL_ERROR)
			md.Para["Flag"]="128"
		}
	case "setChannelToDB":
		flag,err:=strconv.Atoi(md.Para["Flag"])
		if err!=nil{
			retErr = fmt.Errorf("setChannelToDB:flag解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}

		err=setChannelToDB(tmpMap["data"],flag)
		if err!=nil{
			retErr = fmt.Errorf("setChannelToDB操作失败", err, LOG_LEVEL_ERROR)
			md.Para["Flag"]=strconv.Itoa(flag+128)
		}
	case "deleteChannelToDB":
		err=deleteChannelToDB(tmpMap["data"])
		if err!=nil{
			retErr = fmt.Errorf("deleteChannelToDB操作失败", err, LOG_LEVEL_ERROR)
			md.Para["Flag"]="128"
		}
	case "setRtuToDB":
		flag,err:=strconv.Atoi(md.Para["Flag"])
		if err!=nil{
			retErr = fmt.Errorf("setRtuToDB:flag解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}

		err=setRtuToDB(tmpMap["data"],flag)
		if err!=nil{
			retErr = fmt.Errorf("setRtuToDB操作失败", err, LOG_LEVEL_ERROR)
			md.Para["Flag"]=strconv.Itoa(flag+128)
		}
	case "deleteRtuToDB":
		err=deleteRtuToDB(tmpMap["data"])
		if err!=nil{
			retErr = fmt.Errorf("deleteRtuToDB操作失败", err, LOG_LEVEL_ERROR)
			md.Para["Flag"]="128"
		}
	case "getRunningFlag":
		md.Data,err=getRunningFlag(&RealTimeDatabase)
		if err!=nil{
			retErr = fmt.Errorf("getRunningFlag获取运行标志失败", err, LOG_LEVEL_ERROR)
			md.Para["Flag"]="128"
		}
	case "runRTD":
		err=runRTD(&RealTimeDatabase)
		if err!=nil{
			retErr = fmt.Errorf("runRTD启动失败", err, LOG_LEVEL_ERROR)
		}
	case "stopRTD":
		RealTimeDatabase.StopProtocol()
		stopRTD(&RealTimeDatabase)
		LogIt("已停止RTD",nil,LOG_LEVEL_INFO)
	case "updatePointsFromRTD":

		pointType,err:=strconv.Atoi(md.Para["Point_Type"])
		if err!=nil{
			retErr = fmt.Errorf("updatePointsFromRTD:PointType解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
		channelID,err:=strconv.Atoi(md.Para["Channel_ID"])
		if err!=nil{
			retErr = fmt.Errorf("updatePointsFromRTD:channelID解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}
		rtuID,err:=strconv.Atoi(md.Para["Rtu_ID"])
		if err!=nil{
			retErr = fmt.Errorf("updatePointsFromRTD:rtuID解析失败", err, LOG_LEVEL_ERROR)
			return nil, retErr
		}

		md.Data,err=updatePointsFromRTD(channelID,rtuID,pointType)
	//case "getPointsFromRTD"	:
	//	nodeID,err:=strconv.Atoi(md.Para["Node_ID"])
	//
	//
	//
	//	channelID,err:=strconv.Atoi(md.Para["Channel_ID"])
	//	rtuID,err:=strconv.Atoi(md.Para["Rtu_ID"])
	//	pointType,err:=strconv.Atoi(md.Para["Point_Type"])
	//
	//	md.Data,err=getPointsFromRTD(md.Para["Node_ID"],md.Para["Channel_ID"],md.Para["Rtu_ID"],md.Para["Point_Type"])



	default:
		retErr=fmt.Errorf("未知命令")
		return nil,retErr
	}

	retjson,err:=json.Marshal(md)
	LogIt("json解析失败",err,LOG_LEVEL_ERROR)
	return retjson, retErr
}
//构建一个message响应，传入指定的node的发送通道中
func ReplyMessage(request *message,data []byte,sendMessage chan *message)error{
	var response message
	var retErr error
	response.SrcNodeID=request.DesNodeID
	response.SrcAppID=request.DesAppID
	response.DesNodeID=request.SrcNodeID
	response.DesAppID=request.SrcAppID
	response.FunctionCode=request.FunctionCode
	switch request.FunctionCode{
	case 7:
		switch request.Para {
		case 0:
			response.Para=1
			response.Data=data
			sendMessage<-&response
		default:
			retErr=fmt.Errorf("message参数未知")
		}
	case 8:
	default:
		retErr=fmt.Errorf("响应message失败")

	}
	return retErr


}



////////////////////////////////////////////////////////消息总线函数/////////////////////////////////////////////////////////




//获取指定节点的数据库列表
func getNodeList()([]map[string]interface{},error){

	ret:=make([]map[string]interface{},0)
	var retErr error
	local:=make(map[string]interface{},2)
	local["Node_ID"]=BUS.LocalNodeID
	local["Addr"]="LocalNode"
	ret=append(ret,local)
	for _,node:=range BUS.NodeMap{
		tmp:=make(map[string]interface{},2)
		tmp["Node_ID"]=node.ID
		tmp["Addr"]=node.Conn.RemoteAddr().String()
		ret=append(ret,tmp)
	}
	return ret,retErr
}
func getRunningFlag(RTD *MemoryDatabase)(map[string]int,error){
	ret:=make(map[string]int)
	var retErr error
	ret["RunningFlag"]=RTD.Flag
	return ret,retErr
}
func runRTD(RTD *MemoryDatabase)error{
	var retErr error
	err:=RTD.InitMemoryDatabase(&Cfg)
	if err!=nil{
		retErr=fmt.Errorf("初始化实时库失败")
		return retErr
	}
	go RTD.Run()
	return nil
}
func stopRTD(RTD *MemoryDatabase){
	RTD.CmdChannel<-0
	RTD.Flag=0
}






