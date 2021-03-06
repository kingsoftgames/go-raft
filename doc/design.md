## go-raft 设计文档
go-raft 基于hashicorp/raft
### go-raft做了什么
hashicorp/raft已经基本实现了raft整个算法，go-raft只是在原来的基础上封装了一套框架
- 管理各个node，可以配nomad做到自动扩容
- 所有节点均可以接收请求，但是最终会把请求处理转发到leader节点处理
- 支持版本动态迭代升级

### 配置
- 支持配置文件yml(json)，也支持命令行参数（命令行参数优先级最高）
- 配置在文件 app/config.go 中，只需要在结构体 Configure 中加入相应的成员就行， 成员变量需要加入相应的tag标签(yaml,json,help,default)<br>
    从配置文件到flag的实现在 common/flag中，里面用到了go的反射
- 配置还支持 nomad 实时渲染域名的ip地址文件reload，具体可以搜索方法 watchJoinFile

### 代码目录
- app 框架组织代码部分
- common 通用部分代码
- store raft封装部分代码
- cmd 代码生成程序
- inner 节点之间通信的grpc
- test 测试代码目录

### 接口
- 暂时只实现了grpc接口，http部分不用看，没有实现
- grpc 这边 cmd/protoc-gen-go-raft 实现了部分代码生成，可以少写部分冗余代码
- http 本来实现了部分，但是最后不完善，先不用管，后续有机会可以再实现

### 部分实现说明
- 除了real_main中的部分协程使用了裸协程，其他的协程都封装了起来，具体可以看 GracefulGoFunc，app在退出时会跟踪所有协程是否退出，<br>
  如果退出超时，那估计是有某些协程退出有bug，打开配置 EnableGoTrace ，会跟踪协程泄漏信息的日志，建议看下具体实现
  
- raft 落地，raft涉及到两个地方落地，一个是数据快照，一个是日志，启动恢复也是先从数据快照快速恢复，然后再从当前数据快照的日志点恢复，数据快照是定时存储，日志记录的是实时的
  1. 测试阶段发现如果数据和日志都用 Mem 模式，这个速度会是最快的
  2. 快照定时落地到磁盘，日志实时落地到磁盘，这个速度比较慢（但是最安全，即使所有节点一起退出，再次恢复时也不会丢失数据）
  3. 快照定时落地到磁盘，日志保留一个缓冲区（缓冲区大小可配置），缓冲区满或者到一定时间落地到磁盘，这个速度比较快，但是如果所有节点一起异常退出会丢失部分数据，但是如果只是单个节点退出并重启时并不会丢失数据，目前我用的是这种方案（可以根据使用场景来决定）
  
- 上层应用如果需要使用该框架，只需要实现接口 IApp 即可，MainApp会自动注册 IApp 的 grpc 的逻辑处理接口，具体看 app/handle.go 中的 Register

- test 目录下是一些测试用例

### 迭代更新
- 配合nomad支持滚动更新
  1. 如果节点收到exitSignal信号时，先fabio反注册当前节点的服务，不接收新的请求
  2. 如果是leader节点，会transferLeader
  3. 如果是follower节点，处理完剩余的请求
  4. 主协程等待其它所有协程退出时才退出
 
## 编码，优化方面
- defer 是有消耗的，调用频繁的地方可以考虑替换掉defer
- channel 会满的，避免因为压力大时channel满了后阻塞造其他地方连锁反应的假死情况
- 我觉得尽量能跟踪自己创建的所有的协程的生命周期
- 协程
```cassandraql
// 错误用法（这个即使老手也会经常写错，因为有时用法不会写的这么直观）
for i:=0;i<10;i++ {
    go func()
    {
        log.Println(i)
    }()
}
//正确用法
for i:=0;i<10;i++ {
    go func(i int)
    {
        log.Println(i)
    }(i)
}
```
- 我觉得go所有的特性能用上的都能试试，不试试怎么知道有哪些坑

## 启动过程
- 程序在启动之前会调用各个模块的 init 函数注册逻辑
- 初始化 flag，这里包括从 yaml/json 读取配置文件，从flag读取配置
- 调用 MainApp.Init 初始化
- - initDebugConfig 初始化debug信息
- - InitLog 初始化日志信息
- - InitCodec 初始化 Codec 信息
- - IApp.Init 初始化 IApp 信息，应用层逻辑的初始化在这里面调用
- - 注册各种事件处理
- - 调用 RaftStore.Open 初始化 store 
- - - 根据配置初始化落地存盘信息，并创建 Raft
- - - RaftStore.runObserver 启动 raft 状态监控协程
- - - RaftStore.runApply 启动 Apply 协程集，但是如果配置 RaftApplyHash==0 的话，该协程集并不会起作用
- - - RaftStore.runExpire 启动 key 的过期检测定时器，当 KeyExpireS>0 时启用
- - joinSelf 把本节点也一起放到 MemberList 中，
- - 注册 grpc 服务，包括内部grpc通信和外面api的grpc通信
- - runOtherRequest 启用处理内部grpc请求的协程
- - Handler.Register 注册 IApp 的处理函数
- - healthTicker 开启心跳定时器
- - watchJoinFile 监控 join_addr.txt 文件，可以根据这个txt文件动态增加raft节点
- - 初始化prome
- - 注册处理store的Expire事件
- MainApp.Start 开启各路协程处理
- - innerLogic 主要是处理 MainApp 中的定时器回调，watchFile回调
- - runGRpcRequest 处理其他节点发过来的grpc请求
- - runLogic 实际处理Api的协程
- - 启动 fileWatch
- - startUpdateTime 处理各节点直接的心跳超时
- 整个服务器算启动完成并等待退出信号

## 退出过程
- MainApp.preStop 停止prome, 定时器，watch等
- FutureCmdTypeGracefulStop 走Stop逻辑
- - 如果是follower节点，通知leader节点删除自己，处理完当前所有的api请求后正常退出
- - 如果是leader节点，transferLeader后，等待变成follower节点，走follower节点退出流程