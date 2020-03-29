# grpc基础组件简单封装 

## 主要功能
+ 调用链监控
- 性能监控
* 日志采集
+ grpc连接池

### 安装
    go get github.com/happyhakka/grpc-wrapper

### grpc server
	
	s := grpc.NewGrpcServeWrapper()
	s.Init("order-service", "6066")
	order.RegisterOrderServiceServer(s.GetServer(), new(order.OrderServiceImpl))
	s.Run()

### grpc client
	opt := grpc.NewPoolOption("order-service", []string{"127.0.0.1:6066"}, 5, 10)
	pool, err := grpc.NewDefaultGrpcPool(opt)
	if err != nil {
		logger.Error("init grpc-client-pool failed!")
		return
	}
	defer pool.Close()
	conn, err := pool.Get()
	defer pool.Put(conn)


### 日志组件
#### 调用方式
    import (
        . "github.com/happyhakka/grpc-wrapper/log"
        )
        
    //如果指定的log.json不存在，则直接使用默认日志配置，只会打印到控制台    
    logger, err := InitLogger("log.json")
	if err != nil || logger == nil {
		fmt.Printf("init logger fail! error<%v>\n", err)
		return
	}
	
	//直接使用返回日志对象写日志
	logger.Warn("log msg",zap.String("id","uuuid")
	
	//直接使用包里面初始化好的Log写日志
	Log.Info("log xxxx",zap.String("key","value")
	
	//自由格式写日志
	Logf.Infof("format %v", obj)

### grpc拦截器控制开关
#### 控制方式一：
    通过编码设置对应的开关

#### 控制方式二：
通过设置环境变量

##### 设置grpc日志 on|off,默认为on
export env_log_flag=on

##### 设置日志配置文件,默认打印到控制台
export env_log_config_file=conf/log.json

##### 调用链接监控开关 on|off，默认为off
export env_trc_flag=on

##### 设置jaeger服务器地址
export env_trc_addr="127.0.0.1:6831"

##### 打开普罗米修斯性能监控,默认为on
export env_prom_flag=on

##### 设置打开普罗米修斯性能监控端口，默认为5055
export env_prom_addr=":5055"

##### 是否开启客户端重启机制on|off,默认为off
export env_clt_retry_flag=on

##### 重试次数
export env_clt_retry_times=5

##### 重试超时,单位秒
export env_clt_retry_timeout=5
