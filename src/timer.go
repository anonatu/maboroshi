package main
import(
	"time"
	"fmt"
)

type TimerWorker struct{
	WorkerName string
	ReturnChan chan struct{}
	Period int32
	Mode int
	State int
	Tmp int32
}
type TimeFactory struct{
	Period int32
	WorkerChanMap map[string]*TimerWorker
	ControlChan chan int
}

func NewFactory()*TimeFactory {
	t:=&TimeFactory{
		Period: 50,
		WorkerChanMap: make(map[string]*TimerWorker),
		ControlChan: make(chan int),
		}
	return t
}

func (timer *TimeFactory)Run(){

	for _,worker:=range timer.WorkerChanMap{
		worker.Tmp=worker.Period/timer.Period
		//fmt.Println(worker.Tmp)
	}
	for{

		select {
		case <-timer.ControlChan:
		default:

			for _, p := range timer.WorkerChanMap {
				//fmt.Println(p.WorkerName,p.State,p.Tmp)
				if p.State==1 {
					//p.Tmp = p.Period/timer.Period
					if p.Tmp ==0 {

						p.ReturnChan<-struct{}{}

						p.State=0
						//fmt.Println("编号：",ID,"的worker")
					} else {

						p.Tmp -= 1
						//fmt.Println("t2",p)
					}
				}


			}
		}
		time.Sleep(time.Duration(timer.Period*1000000))

	}

}
func (timer *TimeFactory) Hire(workerName string,period int32,mode int,state int)(chan struct{},error){
	//_,ok:=timer.WorkerChanMap[workerName]
	//if ok{
	//	s:=fmt.Errorf("已经存在该id的worker")
	//	return nil,s
	//}
	w:=&TimerWorker{
		WorkerName: workerName,
		ReturnChan: make(chan struct{},1),
		Period: period,
		Mode: mode,
		State: state,
		Tmp:period/timer.Period,
	}
	timer.WorkerChanMap[workerName]=w

	return w.ReturnChan, nil
}
func (timer *TimeFactory)Work(workerName string)error {
	var retErr error
	if _,ok:=timer.WorkerChanMap[workerName];!ok{
		retErr=fmt.Errorf("没有这个worker:%s",workerName)
		return retErr
	}
	timer.WorkerChanMap[workerName].Tmp=0
	timer.WorkerChanMap[workerName].State=1
	return retErr
}
//让worker暂停计时
func (timer *TimeFactory)Rest(workerName string)error{
	var retErr error
	if _,ok:=timer.WorkerChanMap[workerName];!ok{
		retErr=fmt.Errorf("没有这个worker:%s",workerName)
		return retErr
	}
	timer.WorkerChanMap[workerName].State=0
	return retErr
}
//删除worker
func (timer *TimeFactory)Fire(workerName string)error{
	var retErr error
	if _,ok:=timer.WorkerChanMap[workerName];!ok{
		retErr=fmt.Errorf("没有这个worker:%s",workerName)
		return retErr
	}
	delete(timer.WorkerChanMap,workerName)
	return retErr
}
func(timer *TimeFactory)GetChannel(workerName string)(chan struct{},error){
	var retErr error
	if _,ok:=timer.WorkerChanMap[workerName];!ok{
		retErr=fmt.Errorf("没有这个worker:%s",workerName)
		return nil,retErr
	}
	return timer.WorkerChanMap[workerName].ReturnChan, retErr

}




//func main(){
//	timer:=NewFactory()
//	go timer.Run()
//
//	c,err:=timer.Hire(5,10000,5,1)
//	a,err:=timer.Hire(1,5000,5,1)
//
//	t1:=time.Now().UnixNano()
//	go func() {
//		time.Sleep(30*time.Second)
//		timer.Rest(5)
//	}()
//	if err==nil{
//		for {
//			select {
//			case <-c:
//				t2 := time.Now().UnixNano()
//				fmt.Println("t", t1, t2, "误差：", t2-t1)
//				t1 = t2
//			case <-a:
//				fmt.Println("from a")
//			default:
//				time.Sleep(time.Millisecond*2)
//
//			}
//		}
//
//	}else{
//		fmt.Println(err)
//	}
//}
//



