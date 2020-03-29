package main

import (
	"context"
	"fmt"
	_ "net/http/pprof"
	"time"

	"github.com/happyhakka/grpc-wrapper/example/order"
	"github.com/happyhakka/grpc-wrapper/grpc"
	. "github.com/happyhakka/grpc-wrapper/log"

	"go.uber.org/zap"
)

func main() {

	logger, err := InitLogger("log.json")
	if err != nil || logger == nil {
		fmt.Printf("init logger fail! error<%v>\n", err)
		return
	}

	opt := grpc.NewPoolOption("order-service", []string{"127.0.0.1:6066"}, 5, 10)
	pool, err := grpc.NewDefaultGrpcPool(opt)
	if err != nil {
		logger.Error("init grpc-client-pool failed!", zap.Any("error", err))
		return
	}
	defer pool.Close()
	conn, err := pool.Get()
	defer pool.Put(conn)

	clt := order.NewOrderServiceClient(conn)
	req := &order.OrderRequest{OrderId: "201907300001", TimeStamp: time.Now().Unix()}
	rsp, err := clt.GetOrderInfo(context.Background(), req)
	if err != nil {
		logger.Error("call  failed.", zap.String("err", err.Error()))
		return
	}

	Log.Info("应答结果:", zap.Any("rsp", rsp))
	Logf.Warnf("应答结果:%v", rsp)
}
