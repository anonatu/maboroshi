package main

import (
	"net"
	"strconv"
	"time"
)
const(
	MODBUS_FUNCCODE_READ_COIL_STATUS        =1
	MODBUS_FUNCCODE_READ_INPUT_STATUS       =2
	MODBUS_FUNCCODE_READ_HOLDING_REGISTER   =3
	MODBUS_FUNCCODE_READ_INPUT_REGISTER     =4
	MODBUS_FUNCCODE_WRITE_SINGLE_COIL       =5
	MODBUS_FUNCCODE_WRITE_SINGLE_REGISTER   =6
	MODBUS_FUNCCODE_WRITE_MULTIPLE_COIL     =15
	MODBUS_FUNCCODE_WRITE_MULTIPLE_REGISTER =16
)

type ModbusTcpServer struct {

	SlaveAddr       byte
	Conn            net.Conn
	ChanCfg			*ChannelConfig
	TagIndex    	[]map[int]string
	FromRTD			chan int
	cmdChannel		chan int
	listener  		net.Listener
	//ResponseTimeOut time.Duration
}
type MTSMessage struct{
	TransactionID int		//事务处理标识
	ProtocolID int			//协议标识符
	Length int				//长度
	ElementID byte			//单元标识符
	FuncCode byte			//功能码
	Data []byte				//数据
	ExceptionCode int   	//异常码
}


//初始化modbusTcp结构体,从配置中取用需要的参数
func (MTServer *ModbusTcpServer)InitModbusTcpServer(chanCfg *ChannelConfig,rtuCfgList []*RtuConfig)error{
	var err error
	addr:=rtuCfgList[0].Rtu_Addr
	MTServer.SlaveAddr=byte(addr)
	MTServer.listener,err =net.Listen("tcp",":"+strconv.Itoa(chanCfg.Listen_Port))
	LogIt("modbusTcpServer Listen失败,通道号:"+strconv.Itoa(chanCfg.Channel_ID),err,LOG_LEVEL_ERROR)
	if err!=nil{
		return err
	}
	//MTServer.Conn,err=Listener.Accept()
	//LogIt("modbusTcpServer Accept失败,通道号:"+strconv.Itoa(chanCfg.Channel_ID),err,LOG_LEVEL_ERROR)
	//if err!=nil{
	//	return err
	//}

	MTServer.SlaveAddr=byte(chanCfg.Channel_ID)
	MTServer.ChanCfg=chanCfg

	//从实时库中读取目录
	MTServer.TagIndex=make([]map[int]string,5)
	MTServer.TagIndex[MODBUS_FUNCCODE_READ_COIL_STATUS]=make(map[int]string)
	MTServer.TagIndex[MODBUS_FUNCCODE_READ_INPUT_STATUS]=make(map[int]string)
	MTServer.TagIndex[MODBUS_FUNCCODE_READ_HOLDING_REGISTER]=make(map[int]string)
	MTServer.TagIndex[MODBUS_FUNCCODE_READ_INPUT_REGISTER]=make(map[int]string)
	MTServer.cmdChannel=make(chan int)




	for tagName,point:=range RealTimeDatabase.PointsMap[POINT_TYPE_ANALOG]{
		if point.Channel_ID==chanCfg.Channel_ID {
			switch point.Param2{
			case 3:
				MTServer.TagIndex[MODBUS_FUNCCODE_READ_HOLDING_REGISTER][point.Param1]=tagName
			case 4:
				MTServer.TagIndex[MODBUS_FUNCCODE_READ_INPUT_REGISTER][point.Param1]=tagName
			default:
			}

		}
	}
	for tagName,point:=range RealTimeDatabase.PointsMap[POINT_TYPE_DIGITAL]{
		if point.Channel_ID==chanCfg.Channel_ID {
			switch point.Param2{
			case 1:
				MTServer.TagIndex[MODBUS_FUNCCODE_READ_COIL_STATUS][point.Param1]=tagName
			case 2:
				MTServer.TagIndex[MODBUS_FUNCCODE_READ_INPUT_STATUS][point.Param1]=tagName
			default:
			}
		}
	}
	return nil
}
func (MTServer ModbusTcpServer)Run() {
	for {
		var err error
		MTServer.Conn, err= MTServer.listener.Accept()
		if err != nil {
			time.Sleep(10*time.Second)
			continue
		}
readLoop:
		for {
			modbustcpsMessage := make([]byte, 260)
			select {

			case cmd := <-MTServer.cmdChannel:
				switch cmd {
				case 0:
					return
				}
			default:
				n, err := MTServer.Conn.Read(modbustcpsMessage)
				if err!=nil{
					break readLoop
				}
				//t1:=time.Now().UnixNano()
				LogIt("通道号："+strconv.Itoa(MTServer.ChanCfg.Channel_ID)+"接收数据失败", err, LOG_LEVEL_ERROR)
				reply, _ := ModbusTcpProcessor(modbustcpsMessage[:n], &MTServer)
				//fmt.Println("回复",reply)
				_, err = MTServer.Conn.Write(reply)
				//t2:=time.Now().UnixNano()
			}

		}
	}
}
func (MTServer ModbusTcpServer)Stop(){
	MTServer.cmdChannel<-0
}
func ModbusTcpProcessor(received []byte,MTS *ModbusTcpServer)([]byte,error){
	//fmt.Println("收到",received)
	//收到的报文
	var receivedMessage MTSMessage
	//发送的报文
	var replyMessage MTSMessage

	//var commandToRTD string
	//初始化一个点列表
	pointList:=make(map[int]*Point)
	//解析收到的请求报文
	receivedMessage.ParseMTSMessage(received)
	replyMessage.TransactionID=receivedMessage.TransactionID
	replyMessage.ProtocolID=receivedMessage.ProtocolID
	replyMessage.ElementID=MTS.SlaveAddr
	replyMessage.FuncCode=receivedMessage.FuncCode
	switch receivedMessage.FuncCode{
	case MODBUS_FUNCCODE_READ_COIL_STATUS:

		coilAddr:=MakeUpInt(receivedMessage.Data[0:2],0)
		CoilNum:=MakeUpInt(receivedMessage.Data[2:4],0)

		for i:=coilAddr;i<coilAddr+CoilNum;i++{
			pointList[i]=nil
		}
		ret:=MTS.ReadValue(&RealTimeDatabase,pointList,MODBUS_FUNCCODE_READ_COIL_STATUS)


		if ret==0{
			//正常情况
			replyMessage.FuncCode=receivedMessage.FuncCode
			n:=len(pointList)
			//字节数
			l:=n/8
			r:=n%8
			if r>0{
				//如果不足一个字也要凑齐一个字节
				l+=1
			}
			replyMessage.Data=make([]byte,l)
			for i:=0;i<l;i++{
				var b byte
				for j:=0;j<8;j++{
					curser:=coilAddr+8*i+j
					//游标不能超过点地址的最大值
					if curser>=coilAddr+CoilNum{
						continue
					}
					if pointList[curser].Value>0{
						var tmp byte
						tmp=1<<j
						b|=tmp
					}
				}
				replyMessage.Data[i]=b
			}

			return replyMessage.ConstructMTSMessage(),nil


		}else{
			//非正常情况
			replyMessage.ExceptionCode=ret
			return replyMessage.ConstructMTSMessage(),nil
		}


	case MODBUS_FUNCCODE_READ_INPUT_STATUS:
		coilAddr:=MakeUpInt(receivedMessage.Data[0:2],0)
		CoilNum:=MakeUpInt(receivedMessage.Data[2:4],0)

		for i:=coilAddr;i<coilAddr+CoilNum;i++{
			pointList[i]=nil
		}
		ret:=MTS.ReadValue(&RealTimeDatabase,pointList,MODBUS_FUNCCODE_READ_INPUT_STATUS)


		if ret==0{
			//正常情况
			replyMessage.FuncCode=receivedMessage.FuncCode
			n:=len(pointList)
			//字节数
			l:=n/8
			r:=n%8
			if r>0{
				//如果不足一个字也要凑齐一个字节
				l+=1
			}
			replyMessage.Data=make([]byte,l)
			for i:=0;i<l;i++{
				var b byte
				for j:=0;j<8;j++{
					curser:=coilAddr+8*i+j
					//游标不能超过点地址的最大值
					if curser>=coilAddr+CoilNum{
						continue
					}
					if pointList[curser].Value>0{
						var tmp byte
						tmp=1<<j
						b|=tmp
					}
				}
				replyMessage.Data[i]=b
			}

			return replyMessage.ConstructMTSMessage(),nil



		}else{
			//非正常情况
			replyMessage.ExceptionCode=ret
			return replyMessage.ConstructMTSMessage(),nil
		}
	case MODBUS_FUNCCODE_READ_HOLDING_REGISTER:
		coilAddr:=MakeUpInt(receivedMessage.Data[0:2],0)
		CoilNum:=MakeUpInt(receivedMessage.Data[2:4],0)
		for i:=coilAddr;i<coilAddr+CoilNum;i++{
			pointList[i]=nil
		}
		ret:=MTS.ReadValue(&RealTimeDatabase,pointList,MODBUS_FUNCCODE_READ_HOLDING_REGISTER)

		if ret==0{
			//正常情况
			replyMessage.FuncCode=receivedMessage.FuncCode
			n:=len(pointList)
			replyMessage.Data=make([]byte,0,2*n)

			//字节数
			for i:=0;i<n;i++{

				replyMessage.Data=append(replyMessage.Data,DisintegrateIntoBytes(int(pointList[i].Value),2,0)...)
			}
			return replyMessage.ConstructMTSMessage(),nil


		}else{
			//非正常情况
			replyMessage.ExceptionCode=ret
			return replyMessage.ConstructMTSMessage(),nil
		}

	case MODBUS_FUNCCODE_READ_INPUT_REGISTER:
		coilAddr:=MakeUpInt(receivedMessage.Data[0:2],0)
		CoilNum:=MakeUpInt(receivedMessage.Data[2:4],0)
		for i:=coilAddr;i<coilAddr+CoilNum;i++{
			pointList[i]=nil
		}
		ret:=MTS.ReadValue(&RealTimeDatabase,pointList,MODBUS_FUNCCODE_READ_INPUT_REGISTER)

		if ret==0{
			//正常情况
			replyMessage.FuncCode=receivedMessage.FuncCode
			n:=len(pointList)
			replyMessage.Data=make([]byte,0,2*n)

			//字节数
			for i:=0;i<n;i++{

				replyMessage.Data=append(replyMessage.Data,DisintegrateIntoBytes(int(pointList[i].Value),2,0)...)
			}
			return replyMessage.ConstructMTSMessage(),nil


		}else{
			//非正常情况

			replyMessage.ExceptionCode=ret
			return replyMessage.ConstructMTSMessage(),nil
		}
	case MODBUS_FUNCCODE_WRITE_SINGLE_COIL:
		coilAddr:=MakeUpInt(receivedMessage.Data[0:2],0)
		commandCode:=MakeUpInt(receivedMessage.Data[2:4],0)
		replyMessage.ExceptionCode=MTS.WriteSingleValue(&RealTimeDatabase,[]int{coilAddr},MODBUS_FUNCCODE_WRITE_SINGLE_COIL,commandCode)
		return replyMessage.ConstructMTSMessage(),nil

	case MODBUS_FUNCCODE_WRITE_SINGLE_REGISTER:
		coilAddr:=MakeUpInt(receivedMessage.Data[0:2],0)
		targetValue :=MakeUpInt(receivedMessage.Data[2:4],0)
		replyMessage.ExceptionCode=MTS.WriteSingleValue(&RealTimeDatabase,[]int{coilAddr},MODBUS_FUNCCODE_WRITE_SINGLE_REGISTER, targetValue)
		return replyMessage.ConstructMTSMessage(),nil

	case MODBUS_FUNCCODE_WRITE_MULTIPLE_COIL:
		addr:=MakeUpInt(receivedMessage.Data[0:2],0)
		coilNum:=MakeUpInt(receivedMessage.Data[2:4],0)
		valueList:=receivedMessage.Data[5:5+receivedMessage.Data[4]]
		addrList:=make([]int,coilNum)
		for i:=0;i<coilNum;i++{
			addrList[i]=addr+i
		}
		replyMessage.ExceptionCode=MTS.WriteMultipleValue(&RealTimeDatabase,addrList,MODBUS_FUNCCODE_WRITE_MULTIPLE_COIL, valueList)
		if replyMessage.ExceptionCode==0{

			replyMessage.Data=receivedMessage.Data[0:5]

		}
		return replyMessage.ConstructMTSMessage(),nil
	case MODBUS_FUNCCODE_WRITE_MULTIPLE_REGISTER:
	default:


	}
	return []byte{},nil

}



func (MTS ModbusTcpServer)ReadValue(RTD *MemoryDatabase,pointList map[int]*Point, funcCode byte)int{
	defer RTD.RWFlag.Done()
	ret:=0
	RTD.RWFlag.Add(1)
	var pointType int
	switch funcCode{
	case 1:
		pointType=POINT_TYPE_DIGITAL
	case 2:
		pointType=POINT_TYPE_DIGITAL
	case 3:
		pointType=POINT_TYPE_ANALOG
	case 4:
		pointType=POINT_TYPE_ANALOG
	default:
		ret|=32
		return ret
	}
	for addr,_:=range pointList{
		tagName,ok:=MTS.TagIndex[funcCode][addr]
		if !ok{
			ret|=32
			return ret
		}
		pointList[addr]=RTD.PointsMap[pointType][tagName]
		//下面的部分是关于quality的处理，暂时先不处理
		//if RTD.PointsMap[funcCode][tagName].Quality>0{
		//	return RTD.PointsMap[funcCode][tagName].Quality
		//}
	}
	return ret
}
func (MTS ModbusTcpServer) WriteSingleValue(RTD *MemoryDatabase,addrList []int,funcCode byte,cmdCode int)int{
	var ret int
	switch funcCode {
	case MODBUS_FUNCCODE_WRITE_SINGLE_COIL:
		RTD.RWFlag.Add(1)
		defer RTD.RWFlag.Done()
		target,ok:=RTD.PointsMap[POINT_TYPE_DIGITAL][MTS.TagIndex[MODBUS_FUNCCODE_READ_COIL_STATUS][addrList[0]]]
		if !ok{
			ret=2
			return ret
		}
		switch cmdCode{
		case 0:
			target.Value=0
		case 65280:
			target.Value=1

		}
	case MODBUS_FUNCCODE_WRITE_SINGLE_REGISTER:
		RTD.RWFlag.Add(1)
		defer RTD.RWFlag.Done()
		target,ok:=RTD.PointsMap[POINT_TYPE_ANALOG][MTS.TagIndex[MODBUS_FUNCCODE_READ_HOLDING_REGISTER][addrList[0]]]
		if !ok{
			ret=2
			return ret
		}
		target.Value=float64(cmdCode)
		target.Strategy=0
	default:

	}
	return ret
}
func (MTS ModbusTcpServer) WriteMultipleValue(RTD *MemoryDatabase,addrList []int,funcCode byte, valueList []byte)int{
	var ret int
	switch funcCode {
	case MODBUS_FUNCCODE_WRITE_MULTIPLE_COIL:
		for _,addr:=range addrList{
			_,ok:=RTD.PointsMap[POINT_TYPE_DIGITAL][MTS.TagIndex[MODBUS_FUNCCODE_READ_COIL_STATUS][addr]]
			if !ok{
				return 2
			}
		}
		RTD.RWFlag.Add(1)
		defer RTD.RWFlag.Done()
		for index,addr:=range addrList{
			byteIndex:=index/8
			bitIndex:=index%8
			target:=RTD.PointsMap[POINT_TYPE_DIGITAL][MTS.TagIndex[MODBUS_FUNCCODE_READ_COIL_STATUS][addr]]
			target.Value=float64((valueList[byteIndex]>>bitIndex)&1)
			target.Strategy=1
		}
	case MODBUS_FUNCCODE_WRITE_SINGLE_REGISTER:

	default:

	}
	return ret
}
func (reply *MTSMessage)ConstructMTSMessage()[]byte{

	ret :=make([]byte,0,260)
	ret=append(ret,DisintegrateIntoBytes(reply.TransactionID,2,0)...)
	ret=append(ret,DisintegrateIntoBytes(reply.ProtocolID,2,0)...)
	if reply.ExceptionCode>0{
		reply.FuncCode+=128
		reply.Data=[]byte{byte(reply.ExceptionCode)}
	}else {
		ret = append(ret, DisintegrateIntoBytes(len(reply.Data)+3, 2, 0)...)
		reply.Data=append([]byte{byte(len(reply.Data))},reply.Data...)
	}
	ret=append(ret,1)
	ret=append(ret,byte(reply.FuncCode))
	ret=append(ret,reply.Data...)
	return ret

}
func (received *MTSMessage)ParseMTSMessage(bytes []byte){
	received.TransactionID=MakeUpInt(bytes[0:2],0)
	received.ProtocolID=MakeUpInt(bytes[2:4],0)
	received.Length=MakeUpInt(bytes[4:6],0)
	received.ElementID=bytes[6]
	received.FuncCode=bytes[7]
	received.Data=bytes[8:]
}





