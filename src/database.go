package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"math/rand"
	"strconv"

	//"os"
	"sync"
	"time"
)


/*
关于点状态
bit地址由低到高分别是：
			0:OV 值溢出
			1:BL 被封锁
			2:SB 取代
			3:NT 当前值
			4:IV 有效/无效
			5:NO 包含不存在的点
			6:
			7:
 */

var RealTimeDatabase MemoryDatabase
//点类型


const(
	POINT_TYPE_ACCUML=0
	POINT_TYPE_ANALOG=1
	POINT_TYPE_DIGITAL=2
	POINT_TYPE_ANALOG_CTRL=3
	POINT_TYPE_DIGITAL_CTRL=4
)



type MemoryDatabase struct{

	Flag 			int
	PointsMap 		[]map[string]*Point
	ChannelsMap 	map[int]*ChannelConfig
	RtusMap 		map[int]*RtuConfig
	FromBus 		chan *message
	ToBus 			chan *message
	CmdChannel 		chan int	//控制命令
	RWFlag 			sync.WaitGroup

}

type Point struct {
	Tag_Name     string  //点标签名
	Description  string  //点描述
	Device_ID    int  //设备id
	Rtu_ID       int     //Rtu号
	Channel_ID   int     //通道号
	Point_ID	 int	 //点号，保证通道内唯一即可
	Point_Type   int     //点类型
	Value        float64 //值
	Quality      int     //品质
	If_Use_Script int     //是否使用脚本
	Script        int     //脚本号
	Strategy     int     //策略号 1:固定 2:随机变化 3:递增 4:周期变化
	Base_Value   float64 //基值
	Upper_Limit float64  //上限
	Lower_Limit float64  //下限
	Period int64         //周期
	Step_Size float64    //步长
	Use_Soe int          //是否使用Soe
	Param1 int
	Param2 int
	Param3 int
	Param4 int
	Param5 int
	Param6 int
	Param7 int
	Param8 int

	LastUpdateTime int64

}
type ChannelConfig struct{
	Tag_Name    string //标签名
	Description string //描述
	Channel_ID  int    //通道号
	If_Used     int    //使能
	Conn_Type   int    //通道类型 server client com
	//Protocol_ID 0:modbusTcpServer
	Protocol_ID int    //通讯规约id
	Listen_Port int    //监听端口
	Addr1       string //对端地址1
	Addr2       string //
	Addr3       string //
	Addr4       string //
	Baud        int    //波特率
	Parity      int    //校验
	Databit int        //数据位
	Stopbit int        //停止位
	protocol Protocol
}

type RtuConfig struct{
	Tag_Name    string		//
	Description string
	Rtu_ID      int
	Channel_ID  int
	If_Used     int
	Rtu_Addr    int
	Para1       int
	Para2       int
	Para3       int
	Para4       int
}

func (RTD *MemoryDatabase)InitMemoryDatabase(cfg *Config)error{
	RTD.PointsMap =make([]map[string]*Point,5,5)
	RTD.ChannelsMap=make(map[int]*ChannelConfig)
	RTD.RtusMap=make(map[int]*RtuConfig)
	RTD.PointsMap[POINT_TYPE_ACCUML]=make(map[string]*Point)
	RTD.PointsMap[POINT_TYPE_ANALOG]=make(map[string]*Point)
	RTD.PointsMap[POINT_TYPE_DIGITAL]=make(map[string]*Point)
	RTD.PointsMap[POINT_TYPE_ANALOG_CTRL]=make(map[string]*Point)
	RTD.PointsMap[POINT_TYPE_DIGITAL_CTRL]=make(map[string]*Point)
	//RTD.ReadChannel=make(chan map[string]float64)
	//RTD.WriteChannel=make(chan map[string]float64)
	RTD.CmdChannel=make(chan int,3)
	RTD.ReadFromDB(cfg.DatabaseName)
	return nil
}
func (RTD *MemoryDatabase)ReadFromDB(databaseName string)error{

	var retErr error
	database,err:=sql.Open("sqlite3","./database/"+databaseName+".db")

	if err!=nil{
		retErr=fmt.Errorf("读取数据库文件失败")
	}

	accumlRows, err := database.Query("SELECT * FROM Points_Accuml")

	if err!=nil{
		retErr=fmt.Errorf("查询指定表失败")
	}
	for accumlRows.Next() {
		var point Point
		err:= accumlRows.Scan(
			&point.Tag_Name,
			&point.Description,
			&point.Device_ID,
			&point.Rtu_ID,
			&point.Channel_ID,
			&point.Point_Type,
			&point.Value,
			&point.Quality,
			&point.Script,
			&point.Strategy,
			&point.Base_Value,
			&point.Upper_Limit,
			&point.Lower_Limit,
			&point.Period,
			&point.Step_Size,
			&point.Use_Soe,
			&point.Param1,
			&point.Param2,
			&point.Param3,
			&point.Param4,
			&point.Param5,
			&point.Param6,
			&point.Param7,
			&point.Param8,
			&point.Point_ID,
			&point.If_Use_Script,
		)
		point.Value=point.Base_Value
		if err!=nil{

			continue

		}
		RTD.PointsMap[0][point.Tag_Name]=&point


	}



	analogRows,err:=database.Query("SELECT * FROM Points_Analog")
	if err!=nil{
		retErr=fmt.Errorf("查询指定表失败")
	}
	for analogRows.Next() {
		var point Point
		err:= analogRows.Scan(
			&point.Tag_Name,
			&point.Description,
			&point.Device_ID,
			&point.Rtu_ID,
			&point.Channel_ID,
			&point.Point_Type,
			&point.Value,
			&point.Quality,
			&point.Script,
			&point.Strategy,
			&point.Base_Value,
			&point.Upper_Limit,
			&point.Lower_Limit,
			&point.Period,
			&point.Step_Size,
			&point.Use_Soe,
			&point.Param1,
			&point.Param2,
			&point.Param3,
			&point.Param4,
			&point.Param5,
			&point.Param6,
			&point.Param7,
			&point.Param8,
			&point.Point_ID,
			&point.If_Use_Script,
		)
		if err!=nil{
			continue
		}
		point.Value=point.Base_Value
		RTD.PointsMap[1][point.Tag_Name]=&point

	}
	digitalRows,err:=database.Query("SELECT * FROM Points_Digital")
	if err!=nil{
		retErr=fmt.Errorf("查询指定表失败")
	}
	for digitalRows.Next() {
		var point Point
		err:= digitalRows.Scan(
			&point.Tag_Name,
			&point.Description,
			&point.Device_ID,
			&point.Rtu_ID,
			&point.Channel_ID,
			&point.Point_Type,
			&point.Value,
			&point.Quality,
			&point.Script,
			&point.Strategy,
			&point.Base_Value,
			&point.Upper_Limit,
			&point.Lower_Limit,
			&point.Period,
			&point.Step_Size,
			&point.Use_Soe,
			&point.Param1,
			&point.Param2,
			&point.Param3,
			&point.Param4,
			&point.Param5,
			&point.Param6,
			&point.Param7,
			&point.Param8,
			&point.Point_ID,
			&point.If_Use_Script,
		)
		if err!=nil{
			continue
		}
		point.Value=point.Base_Value
		RTD.PointsMap[2][point.Tag_Name]=&point

	}
	analogCtrlRows,err:=database.Query("SELECT * FROM Points_AnalogCtrl")
	if err!=nil{
		retErr=fmt.Errorf("查询指定表失败")
	}
	for analogCtrlRows.Next() {
		var point Point
		err:= analogCtrlRows.Scan(
			&point.Tag_Name,
			&point.Description,
			&point.Device_ID,
			&point.Rtu_ID,
			&point.Channel_ID,
			&point.Point_Type,
			&point.Value,
			&point.Quality,
			&point.Script,
			&point.Strategy,
			&point.Base_Value,
			&point.Upper_Limit,
			&point.Lower_Limit,
			&point.Period,
			&point.Step_Size,
			&point.Use_Soe,
			&point.Param1,
			&point.Param2,
			&point.Param3,
			&point.Param4,
			&point.Param5,
			&point.Param6,
			&point.Param7,
			&point.Param8,
			&point.Point_ID,
			&point.If_Use_Script,
		)
		point.Value=point.Base_Value
		if err!=nil{
			continue
		}
		RTD.PointsMap[3][point.Tag_Name]=&point

	}
	digitalCtrlRows,err:=database.Query("SELECT * FROM Points_DigitalCtrl")
	if err!=nil{
		retErr=fmt.Errorf("查询指定表失败")
	}
	for digitalCtrlRows.Next() {
		var point Point
		err:= digitalCtrlRows.Scan(
			&point.Tag_Name,
			&point.Description,
			&point.Device_ID,
			&point.Rtu_ID,
			&point.Channel_ID,
			&point.Point_Type,
			&point.Value,
			&point.Quality,
			&point.Script,
			&point.Strategy,
			&point.Base_Value,
			&point.Upper_Limit,
			&point.Lower_Limit,
			&point.Period,
			&point.Step_Size,
			&point.Use_Soe,
			&point.Param1,
			&point.Param2,
			&point.Param3,
			&point.Param4,
			&point.Param5,
			&point.Param6,
			&point.Param7,
			&point.Param8,
			&point.Point_ID,
			&point.If_Use_Script,
		)
		if err!=nil{
			continue
		}
		point.Value=point.Base_Value
		RTD.PointsMap[4][point.Tag_Name]=&point

	}
	//查询channel列表
	channelRows,err:=database.Query("SELECT * FROM Channel_Para")
	if err!=nil{
		retErr=fmt.Errorf("查询指定表失败")
	}
	for channelRows.Next() {
		var channelCfg ChannelConfig
		err:= channelRows.Scan(
			&channelCfg.Tag_Name,
			&channelCfg.Description,
			&channelCfg.Channel_ID,
			&channelCfg.If_Used,
			&channelCfg.Conn_Type,
			&channelCfg.Protocol_ID,
			&channelCfg.Listen_Port,
			&channelCfg.Addr1,
			&channelCfg.Addr2,
			&channelCfg.Addr3,
			&channelCfg.Addr4,
			&channelCfg.Baud,
			&channelCfg.Parity,
			&channelCfg.Databit,
			&channelCfg.Stopbit,
		)
		if err!=nil{

			continue
		}
		RTD.ChannelsMap[channelCfg.Channel_ID]=&channelCfg

	}
	RtuRows,err:=database.Query("SELECT * FROM Rtu_Para")
	if err!=nil{
		retErr=fmt.Errorf("查询指定表失败")
	}
	for RtuRows.Next() {
		var RtuCfg RtuConfig
		err:= RtuRows.Scan(
			&RtuCfg.Tag_Name,
			&RtuCfg.Description,
			&RtuCfg.Rtu_ID,
			&RtuCfg.Channel_ID,
			&RtuCfg.If_Used,
			&RtuCfg.Rtu_Addr,
			&RtuCfg.Para1,
			&RtuCfg.Para2,
			&RtuCfg.Para3,
			&RtuCfg.Para4,
		)
		if err!=nil{
			continue
		}
		RTD.RtusMap[RtuCfg.Rtu_ID]=&RtuCfg

	}
	return retErr

}


func (RTD *MemoryDatabase)Run() {
	RTD.Flag=1
	for _, channelCfg :=range RTD.ChannelsMap{
		var rtuCfgList=make([]*RtuConfig,0)
		for _, rtuCfg :=range RTD.RtusMap{
			if rtuCfg.Channel_ID==channelCfg.Channel_ID{
				rtuCfgList=append(rtuCfgList, rtuCfg)
			}
		}

		switch channelCfg.Protocol_ID{
		case 0:
			var mTS ModbusTcpServer
			if len(rtuCfgList)>1{
				LogIt("通道下挂载的rtu超过一个",nil,LOG_LEVEL_WARN)
				continue

			}
			err:=mTS.InitModbusTcpServer(channelCfg,rtuCfgList)
			if err!=nil{
				LogIt("初始化modbusTcpServer失败",err,LOG_LEVEL_ERROR)
				continue
			}
			channelCfg.protocol=mTS
			go channelCfg.protocol.Run()

		default:
			continue
		}
	}
	for {
		//var flag float64
		select {

		case cmd:=<-RTD.CmdChannel:
			switch cmd{

			case 0:
				RTD.Flag=0
				//停止运行
				return
			default:

			}

		default:
			RTD.RWFlag.Wait()
			UpdateRTD()
			time.Sleep(time.Millisecond*10)


		}

	}
}

func UpdateRTD(){

	now := time.Now().Unix()
	for _, point := range RealTimeDatabase.PointsMap[POINT_TYPE_ACCUML] {
		switch point.Strategy {
		case 2:
			//随机数
			if now-point.LastUpdateTime < point.Period {
				continue
			}

			point.Value = float64(rand.Int63n(int64((point.Upper_Limit-point.Lower_Limit)*100+point.Lower_Limit*100))) / 100

		case 3:
			//递增

			if now-point.LastUpdateTime < point.Period {
				continue
			}
			point.Value += point.Step_Size
		default:

		}
		point.LastUpdateTime = now
	}
		for _, point := range RealTimeDatabase.PointsMap[POINT_TYPE_ANALOG] {
			switch point.Strategy {
			case 2:
				//随机数
				if now-point.LastUpdateTime < point.Period {
					continue
				}
				if point.Upper_Limit-point.Lower_Limit<0{
					return
				}
				point.Value = float64(rand.Int63n(int64((point.Upper_Limit-point.Lower_Limit)*100+point.Lower_Limit*100))) / 100
			case 3:
				//递增

				if now-point.LastUpdateTime < point.Period {
					continue
				}
				point.Value += point.Step_Size
			default:

			}
			point.LastUpdateTime = now



		}

		for _, point := range RealTimeDatabase.PointsMap[POINT_TYPE_DIGITAL] {

			switch point.Strategy {
			case 4:
				//周期
				if now-point.LastUpdateTime < point.Period {
					continue
				}
				if point.Value>0{
					point.Value=0
				}else{
					point.Value=1
				}


			default:

			}
			point.LastUpdateTime=now


		}

}
//读取指定数据库，指定点类型，指定通道指定rtu的点信息
func getPointsFromDB(pointType int,channelID int,rtuID int)(interface{},error){
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)
	pointList:=make([]*Point,0)
	tableNameList:=[]string{"Points_Accuml","Points_Analog","Points_Digital","Points_DigitalCtrl","Points_AnalogCtrl"}
	var retErr error
	str:=fmt.Sprintf("SELECT * FROM %s WHERE Channel_ID=%s AND Rtu_ID=%s",
		tableNameList[pointType],
		strconv.Itoa(channelID),
		strconv.Itoa(rtuID))

	row,err:=db.Query(str)
	if err!=nil{
		retErr=fmt.Errorf("查询数据库失败")
		return nil,retErr
	}
	for row.Next(){
		var point Point
		err:=row.Scan(
			&point.Tag_Name,
			&point.Description,
			&point.Device_ID,
			&point.Rtu_ID,
			&point.Channel_ID,
			&point.Point_Type,
			&point.Value,
			&point.Quality,
			&point.Script,
			&point.Strategy,
			&point.Base_Value,
			&point.Upper_Limit,
			&point.Lower_Limit,
			&point.Period,
			&point.Step_Size,
			&point.Use_Soe,
			&point.Param1,
			&point.Param2,
			&point.Param3,
			&point.Param4,
			&point.Param5,
			&point.Param6,
			&point.Param7,
			&point.Param8,
			&point.Point_ID,
			&point.If_Use_Script,

			)
		if err!=nil{
			LogIt("扫描数据库点信息失败",err,LOG_LEVEL_ERROR)
		}else{
			pointList=append(pointList,&point)
		}
	}
	return pointList,nil
}
func setPointToDB(data interface{},flag int)error{
	var retErr error
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)

	tableNameList:=[]string{"Points_Accuml","Points_Analog","Points_Digital","Points_DigitalCtrl","Points_AnalogCtrl"}
	dataMap:=data.(map[string]interface{})
	pointType:=int(dataMap["Point_Type"].(float64))
	switch flag {
	case 1:
		str := fmt.Sprintf("INSERT INTO %s(Tag_Name,Description,Device_ID,Rtu_ID,Channel_ID,Point_Type,Value,Quality,Script,Strategy,Base_Value,Upper_Limit,Lower_Limit,Period,Step_Size,Use_Soe,Param1,Param2,Param3,Param4,Param5,Param6,Param7,Param8,Point_ID,If_Use_Script) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			tableNameList[pointType])

		stmt, err := db.Prepare(str)
		if err != nil {
			retErr = fmt.Errorf("数据库语句拼接失败")
			return retErr
		}
		_, err = stmt.Exec(
			dataMap["Tag_Name"].(string),
			dataMap["Description"].(string),
			int(dataMap["Device_ID"].(float64)),
			int(dataMap["Rtu_ID"].(float64)),
			int(dataMap["Channel_ID"].(float64)),
			int(dataMap["Point_Type"].(float64)),
			dataMap["Value"].(float64),
			int(dataMap["Quality"].(float64)),
			int(dataMap["Script"].(float64)),
			int(dataMap["Strategy"].(float64)),
			dataMap["Base_Value"].(float64),
			dataMap["Upper_Limit"].(float64),
			dataMap["Lower_Limit"].(float64),
			dataMap["Period"].(float64),
			dataMap["Step_Size"].(float64),
			int(dataMap["Use_Soe"].(float64)),
			int(dataMap["Param1"].(float64)),
			int(dataMap["Param2"].(float64)),
			int(dataMap["Param3"].(float64)),
			int(dataMap["Param4"].(float64)),
			int(dataMap["Param5"].(float64)),
			int(dataMap["Param6"].(float64)),
			int(dataMap["Param7"].(float64)),
			int(dataMap["Param8"].(float64)),
			int(dataMap["Point_ID"].(float64)),
			int(dataMap["If_Use_Script"].(float64)),
		)
		if err!=nil{
			retErr=fmt.Errorf("查询数据库失败")
			return retErr
		}
	case 5:
		str := fmt.Sprintf("UPDATE %s SET Description=?,Device_ID=?,Rtu_ID=?,Channel_ID=?,Point_Type=?,Value=?,Quality=?,Script=?,Strategy=?,Base_Value=?,Upper_Limit=?,Lower_Limit=?,Period=?,Step_Size=?,Use_Soe=?,Param1=?,Param2=?,Param3=?,Param4=?,Param5=?,Param6=?,Param7=?,Param8=?,Point_ID=?,If_Use_Script=? WHERE Tag_Name=?",
			tableNameList[pointType])

		stmt, err := db.Prepare(str)
		if err != nil {
			retErr = fmt.Errorf("数据库语句拼接失败")
			return retErr
		}
		_, err = stmt.Exec(
			dataMap["Description"].(string),
			int(dataMap["Device_ID"].(float64)),
			int(dataMap["Rtu_ID"].(float64)),
			int(dataMap["Channel_ID"].(float64)),
			int(dataMap["Point_Type"].(float64)),
			dataMap["Value"].(float64),
			int(dataMap["Quality"].(float64)),
			int(dataMap["Script"].(float64)),
			int(dataMap["Strategy"].(float64)),
			dataMap["Base_Value"].(float64),

			dataMap["Upper_Limit"].(float64),
			dataMap["Lower_Limit"].(float64),
			dataMap["Period"].(float64),
			dataMap["Step_Size"].(float64),
			int(dataMap["Use_Soe"].(float64)),
			int(dataMap["Param1"].(float64)),
			int(dataMap["Param2"].(float64)),
			int(dataMap["Param3"].(float64)),
			int(dataMap["Param4"].(float64)),
			int(dataMap["Param5"].(float64)),
			int(dataMap["Param6"].(float64)),
			int(dataMap["Param7"].(float64)),
			int(dataMap["Param8"].(float64)),
			int(dataMap["Point_ID"].(float64)),
			int(dataMap["If_Use_Script"].(float64)),
			dataMap["Tag_Name"].(string),
		)
		if err!=nil{
			retErr=fmt.Errorf("查询数据库失败")
			return retErr
		}
	default:
		retErr=fmt.Errorf("flag错误")
	}
	return retErr
}
func deletePointToDB(pointList interface{},pointType int)error{
	var retErr error
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)
	tableNameList:=[]string{"Points_Accuml","Points_Analog","Points_Digital","Points_DigitalCtrl","Points_AnalogCtrl"}
	dataMap:=pointList.([]interface{})
	str:=fmt.Sprintf("DELETE FROM %s WHERE Tag_Name=?",
		tableNameList[pointType])
	//str:=fmt.Sprintf("INSERT INTO %s(Tag_Name),values(?)",
	//	tableNameList[pointType])
	stmt,err:=db.Prepare(str)
	if err!=nil{
		retErr=fmt.Errorf("数据库语句拼接失败")
		return retErr
	}
	for _,point :=range dataMap {
		_, err = stmt.Exec(point.(map[string]interface{})["Tag_Name"].(string))
		if err != nil {
			retErr = fmt.Errorf("数据库删除操作失败")
			return retErr
		}
	}
	return retErr
}
func setChannelToDB(data interface{},flag int)error{
	var retErr error
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)

	tableNameList:="Channel_Para"
	dataMap:=data.(map[string]interface{})
	switch flag {
	case 1:
		str := fmt.Sprintf("INSERT INTO %s(Tag_Name,Description,Channel_ID,If_Used,Conn_Type,Protocol_ID,Listen_Port,Addr1,Addr2,Addr3,Addr4,Baud,Parity,Databit,Stopbit) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			tableNameList)

		stmt, err := db.Prepare(str)
		if err != nil {
			retErr = fmt.Errorf("数据库语句拼接失败")
			return retErr
		}
		_, err = stmt.Exec(
			dataMap["Tag_Name"].(string),
			dataMap["Description"].(string),
			int(dataMap["Channel_ID"].(float64)),
			int(dataMap["If_Used"].(float64)),
			int(dataMap["Conn_Type"].(float64)),
			int(dataMap["Protocol_ID"].(float64)),
			int(dataMap["Listen_Port"].(float64)),
			dataMap["Addr1"].(string),
			dataMap["Addr2"].(string),
			dataMap["Addr3"].(string),
			dataMap["Addr4"].(string),
			int(dataMap["Baud"].(float64)),
			int(dataMap["Parity"].(float64)),
			int(dataMap["Databit"].(float64)),
			int(dataMap["Stopbit"].(float64)),

		)
		if err!=nil{
			retErr=fmt.Errorf("查询数据库失败")
			return retErr
		}
	case 5:
		str := fmt.Sprintf("UPDATE %s SET Description=?,Channel_ID=?,If_Used=?,Conn_Type=?,Protocol_ID=?,Listen_Port=?,Addr1=?,Addr2=?,Addr3=?,Addr4=?,Baud=?,Parity=?,Databit=?,Stopbit=? WHERE Tag_Name=?",
			tableNameList)

		stmt, err := db.Prepare(str)
		if err != nil {
			retErr = fmt.Errorf("数据库语句拼接失败")
			return retErr
		}
		_, err = stmt.Exec(
			dataMap["Description"].(string),
			int(dataMap["Channel_ID"].(float64)),
			int(dataMap["If_Used"].(float64)),
			int(dataMap["Conn_Type"].(float64)),
			int(dataMap["Protocol_ID"].(float64)),
			int(dataMap["Listen_Port"].(float64)),
			dataMap["Addr1"].(string),
			dataMap["Addr2"].(string),
			dataMap["Addr3"].(string),
			dataMap["Addr4"].(string),
			int(dataMap["Baud"].(float64)),
			int(dataMap["Parity"].(float64)),
			int(dataMap["Databit"].(float64)),
			int(dataMap["Stopbit"].(float64)),
			dataMap["Tag_Name"].(string),
		)
		if err!=nil{
			retErr=fmt.Errorf("查询数据库失败")
			return retErr
		}
	default:
		retErr=fmt.Errorf("flag错误")
	}
	return retErr
}
func deleteChannelToDB(channelList interface{})error{
	var retErr error
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)
	tableNameList:="Channel_Para"
	dataMap:= channelList.([]interface{})
	str:=fmt.Sprintf("DELETE FROM %s WHERE Tag_Name=?",
		tableNameList)
	//str:=fmt.Sprintf("INSERT INTO %s(Tag_Name),values(?)",
	//	tableNameList[pointType])
	stmt,err:=db.Prepare(str)
	if err!=nil{
		retErr=fmt.Errorf("数据库语句拼接失败")
		return retErr
	}
	for _, channel :=range dataMap {
		_, err = stmt.Exec(channel.(map[string]interface{})["Tag_Name"].(string))
		if err != nil {
			retErr = fmt.Errorf("数据库删除操作失败")
			return retErr
		}
	}
	return retErr
}
func setRtuToDB(data interface{},flag int)error{
	var retErr error
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)

	tableNameList:="Rtu_Para"
	dataMap:=data.(map[string]interface{})
	switch flag {
	case 1:
		str := fmt.Sprintf("INSERT INTO %s(Tag_Name,Description,Rtu_ID,Channel_ID,If_Used,Rtu_Addr,Para1,Para2,Para3,Para4) values(?,?,?,?,?,?,?,?,?,?)",
			tableNameList)
		stmt, err := db.Prepare(str)
		if err != nil {
			retErr = fmt.Errorf("数据库语句拼接失败")
			return retErr
		}
		_, err = stmt.Exec(
			dataMap["Tag_Name"].(string),
			dataMap["Description"].(string),
			int(dataMap["Rtu_ID"].(float64)),
			int(dataMap["Channel_ID"].(float64)),
			int(dataMap["If_Used"].(float64)),
			int(dataMap["Rtu_Addr"].(float64)),
			int(dataMap["Para1"].(float64)),
			int(dataMap["Para2"].(float64)),
			int(dataMap["Para3"].(float64)),
			int(dataMap["Para4"].(float64)),

		)
		if err!=nil{
			retErr=fmt.Errorf("查询数据库失败")
			return retErr
		}
	case 5:
		str := fmt.Sprintf("UPDATE %s SET Description=?,Rtu_ID=?,Channel_ID=?,If_Used=?,Rtu_Addr=?,Para1=?,Para2=?,Para3=?,Para4=? WHERE Tag_Name=?",
			tableNameList)

		stmt, err := db.Prepare(str)
		if err != nil {
			retErr = fmt.Errorf("数据库语句拼接失败")
			return retErr
		}
		_, err = stmt.Exec(
			dataMap["Description"].(string),
			int(dataMap["Rtu_ID"].(float64)),
			int(dataMap["Channel_ID"].(float64)),
			int(dataMap["If_Used"].(float64)),
			int(dataMap["Rtu_Addr"].(float64)),
			int(dataMap["Para1"].(float64)),
			int(dataMap["Para2"].(float64)),
			int(dataMap["Para3"].(float64)),
			int(dataMap["Para4"].(float64)),
			dataMap["Tag_Name"].(string),
		)
		if err!=nil{
			retErr=fmt.Errorf("查询数据库失败")
			return retErr
		}
	default:
		retErr=fmt.Errorf("flag错误")
	}
	return retErr
}
func deleteRtuToDB(rtu interface{})error{
	var retErr error
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)
	tableNameList:="Rtu_Para"
	dataMap:= rtu.([]interface{})
	str:=fmt.Sprintf("DELETE FROM %s WHERE Tag_Name=?",
		tableNameList)
	//str:=fmt.Sprintf("INSERT INTO %s(Tag_Name),values(?)",
	//	tableNameList[pointType])
	stmt,err:=db.Prepare(str)
	if err!=nil{
		retErr=fmt.Errorf("数据库语句拼接失败")
		return retErr
	}
	for _, v :=range dataMap {
		_, err = stmt.Exec(v.(map[string]interface{})["Tag_Name"].(string))
		if err != nil {
			retErr = fmt.Errorf("数据库删除操作失败")
			return retErr
		}
	}
	return retErr
}

//读取指定数据库名字的通道列表
func getChannelsFromDB()(interface{},error){
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	LogIt("打开数据库文件失败",err,LOG_LEVEL_ERROR)
	chanList :=make([]*ChannelConfig,0)
	row,err:=db.Query("SELECT * FROM Channel_Para")
	if err!=nil{
		return []byte{},err
	}
	for row.Next(){
		var channelCfg ChannelConfig
		err:=row.Scan(
			&channelCfg.Tag_Name,
			&channelCfg.Description,
			&channelCfg.Channel_ID,
			&channelCfg.If_Used,
			&channelCfg.Conn_Type,
			&channelCfg.Protocol_ID,
			&channelCfg.Listen_Port,
			&channelCfg.Addr1,
			&channelCfg.Addr2,
			&channelCfg.Addr3,
			&channelCfg.Addr4,
			&channelCfg.Baud,
			&channelCfg.Parity,
			&channelCfg.Databit,
			&channelCfg.Stopbit,

		)
		if err!=nil{
			LogIt("扫描数据库点信息失败",err,LOG_LEVEL_ERROR)
		}else{
			chanList =append(chanList,&channelCfg)
		}
	}


	return chanList,nil

}
func getRtusFromDB(nodeID,channelID int)(interface{},error){
	var retErr error
	rtuSlice :=make([]*RtuConfig,0)
	db,err:=sql.Open("sqlite3","./database/"+Cfg.DatabaseName+".db")
	defer db.Close()
	if err!=nil{
		retErr=fmt.Errorf("打开数据库文件失败")
		return nil,retErr
	}
	rtuList,err:=db.Query("SELECT * FROM Rtu_Para")
	if err!=nil{
		retErr=fmt.Errorf("打开数据库文件失败")
		return nil,retErr
	}
	for rtuList.Next(){
		var rtuCfg RtuConfig
		rtuList.Scan(
			&rtuCfg.Tag_Name,
			&rtuCfg.Description,
			&rtuCfg.Rtu_ID,
			&rtuCfg.Channel_ID,
			&rtuCfg.If_Used,
			&rtuCfg.Rtu_Addr,
			&rtuCfg.Para1,
			&rtuCfg.Para2,
			&rtuCfg.Para3,
			&rtuCfg.Para4,
			)
		if rtuCfg.Channel_ID ==channelID {
			rtuSlice = append(rtuSlice, &rtuCfg)
		}

	}

	return rtuSlice,retErr
}
//读取数据库的列表
func getDatabaseList()([]byte,error){
	 var ret []byte
	 var retErr error
	fileNameList:=make([]string,0)
	filenames,err:=ioutil.ReadDir("./database/")
	if err!=nil{
		retErr=fmt.Errorf("读取数据库目录失败")
	}
	for _,fileInfor:=range filenames{
		if fileInfor.IsDir(){
			continue
		}
		fileNameList=append(fileNameList,fileInfor.Name())
	}
	ret,err=json.Marshal(fileNameList)
	if err!=nil{
		retErr=fmt.Errorf("json解析失败")
	}
	return ret,retErr

}
func getPointType()(interface{},error){
	ret:=make([]map[string]interface{},5)
	ret=[]map[string]interface{}{
		map[string]interface{}{"PointType_ID": POINT_TYPE_ACCUML, "Description": "ACCUML"},
		map[string]interface{}{"PointType_ID": POINT_TYPE_ANALOG, "Description": "ANALOG"},
		map[string]interface{}{"PointType_ID": POINT_TYPE_DIGITAL, "Description": "DIGITAL"},
		map[string]interface{}{"PointType_ID":POINT_TYPE_ANALOG_CTRL, "Description":"ANALOG_CTRL"},
		map[string]interface{}{"PointType_ID":POINT_TYPE_DIGITAL_CTRL, "Description":"DIGITAL_CTRL"},
	}
	return ret,nil
}
//获取实时库的点，需传入通道ID、RTUID、点类型
//func getPointsFromRealtimeDB(channelID ,rtuID,pointType int)([]byte,error){
//	var ret []byte
//	var retErr error
//	tmpPointsList:=make(map[int]Point)
//	defer RealTimeDatabase.RWFlag.Done()
//	RealTimeDatabase.RWFlag.Add(1)
//	for _,point:=range RealTimeDatabase.PointsMap[pointType]{
//		if point.Channel_ID==channelID&&point.Rtu_ID==rtuID{
//			tmpPointsList[point.Point_ID]=*point
//		}
//
//	}
//	ret,err:=json.Marshal(tmpPointsList)
//	if err!=nil{
//		retErr=fmt.Errorf("json解析失败")
//	}
//	return ret ,retErr
//}
func getPointsFromRTD(channelID ,rtuID ,pointType int)(interface{},error){
	ret:=make([]*Point,0)
	var retErr error
	for _,point:=range RealTimeDatabase.PointsMap[pointType]{
		if point.Rtu_ID==rtuID&&point.Channel_ID==channelID{
			ret=append(ret, point)
		}
	}
	return ret,retErr
}
//用来从实时库中更新部分参数 目前只是value
func updatePointsFromRTD(channelID ,rtuID ,pointType int)(interface{},error){
	ret:=make([]map[string]interface{},0)
	var retErr error
	for _,point:=range RealTimeDatabase.PointsMap[pointType]{
		if point.Rtu_ID==rtuID&&point.Channel_ID==channelID{
			tmpMap:=make(map[string]interface{})
			tmpMap["Tag_Name"]=point.Tag_Name
			tmpMap["Value"]=point.Value
			ret=append(ret, tmpMap)
		}
	}
	return ret,retErr
}
func (RTD *MemoryDatabase)StopProtocol(){
	for _,channel:=range RTD.ChannelsMap{
		channel.protocol.Stop()
	}
	LogIt("通道通讯停止运行",nil,LOG_LEVEL_INFO)
}