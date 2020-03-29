package main

import (
	"github.com/happyhakka/grpc-wrapper/example/order"
	"github.com/happyhakka/grpc-wrapper/grpc"
)

func main() {

	s := grpc.NewGrpcServeWrapper()
	s.Init("order-service", "6066")
	order.RegisterOrderServiceServer(s.GetServer(), new(order.OrderServiceImpl))
	s.Run()
}
