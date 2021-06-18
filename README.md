# maboroshi
A communication protocol simulation platform by Golang
简单的一个通讯规约仿真平台，通过web界面进行规约的配置，具备在局域网内节点发现的功能。目前只做了modbusTcpServer规约的部分功能，节点自动发现的功能目前虽然能够发现，但是这个部分bug实在太多，目前只建议单节点使用，这个软件暂时只能保证跑起来，实现简单的通讯功能，仅供有感兴趣的人作为参考使用，编码和配置流程仍需大幅度优化，最近刚好离职且还没找工作就把过去工作做的一些小工具和想法汇总起来做了个小东西，但是过几天要开始面试,可能后续没有没有精力进行太大的功能增加，只能先发出来个简单版本留待维护
平台运行的参数通过config.json进行配置，参数的说明如下：
  +"DatabaseName":"maboroshi",                  |数据库名字，这个不要改
  +"WebStarUp":true,                            |是否启动web，这个不要改
  +"WebServerAddr":"127.0.0.1:80",              |web访问的地址，ip可以修改，80不能修改
  +"NodeDiscoveryMode":0,                       |节点发现模式，这个不要改，今后计划更新使用三种模式：单节点模式，多节点预配置模式，节点自动发现模式
  +"UDPServerAddr":"192.168.1.10:10800",        |UDPserver地址，需配置
  +"UDPClientAddr":"192.168.1.10:11800",        |UDPclient地址，需配置
  +"TCPServerAddr":"192.168.1.10:12800",        |TCPserver地址，需配置
  +"TCPClientAddr":"192.168.1.10:13000",        |TCPclient地址，需配置
  +"PresetRemoteNodes":{"1":"127.0.0.1:13800"}, |这个是可能用预置节点配置信息，暂时没用
  +"PresetLocalNodeID":0,                       |这个先不要改，用作以后和预置节点模式配合使用
  +"BusUpdateCycle":500                         |节点自动发现模式下，每隔一段时间就会通过udp广播向其他外部节点公示自己的信息，这里就是配置间隔的地方（单位秒），建议不要设置得过短

大致说一下配置的流程
  1.首先修改config.json
  2.启动程序
  3.访问http://ip:port/database/channels,(说来惭愧，web只有简单的表格配置，没有做界面的美化，主页也没来得及做，好在基本功能都已经实现)
  3.进行通道配置
    1.点击刷新channel信息，刷新出local节点的信息之后，点击添加channel信息
    2.对于mdbusTCPServer规约需要关注几个参数
      +Description：描述信息而已，在之后配置中方便区分即可
      +channnel_ID：这个是作为软件内部的一个标识，与规约无关，但是要保证节点内唯一
      +Protoco_ID:填写0就表示是使用mdbusTCPServer规约，这个之后的更新中会加上规约的描述信息然后改为select的形式
      +Listen_Port:这个是规约listen的端口，基于tcp的北向协议基本都要配置这个
  4.点击rtu配置，点击刷新节点信息，选中刚刚建立的channel，在这个channel中添加rtu，需要注意的几个参数如下：
      +Description：描述信息而已，在之后配置中方便区分即可
      +Rtu_ID：这个是作为软件内部的一个标识，与规约无关，但是要保证节点内唯一
      +Rtu_Addr:就是modbus子节点的地址
  5.点击点配置，再点击刷新节点信息之后选择上面配置的那些channel和Rtu，选择要浏览和修改的点类型，注意，这里大致了几种点的类型，modbus用到的大概只有模拟量和数字量两种的读和写。需要注意的参数如下
      +Description：描述信息而已，在之后配置中方便区分即可
      +Base_Value:仿真点的基础值
      +Strategy：数据点的自动变化策略 1：固定不变，2：随机变化，3：递增，4：周期变化
      +Step_Size：变化步长，与自动变化策略配合，当递增变化时候，每个周期就会增加的步长
      +Period：变化周期与自动变化策略配合
      +Upper_LImit：数值变化的上限
      +Lower_Limit：数值变化的下限
      +Param1：modbus的寄存器或者线圈地址
      +Param2：读点的功能码，模拟量可以填写3或者4，数字量对应1或2
  6.点击运行
