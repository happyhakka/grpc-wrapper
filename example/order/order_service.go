package order

import (
	"context"
	fmt "fmt"

	"sync"

	//"google.golang.org/grpc/codes"
	//metadata "google.golang.org/grpc/metadata"
	"errors"
	"time"
)

type OrderServiceImpl struct {
}

var (
	lock     *sync.RWMutex
	orderMap map[string]OrderInfo
)

func init() {
	lock = &sync.RWMutex{}
	lock.Lock()
	defer lock.Unlock()
	orderMap = map[string]OrderInfo{
		"201907300001": OrderInfo{OrderId: "201907300001", OrderName: "衣服", OrderStatus: "已付款"},
		"201907310001": OrderInfo{OrderId: "201907310001", OrderName: "零食", OrderStatus: "已付款"},
		"201907310002": OrderInfo{OrderId: "201907310002", OrderName: "食品", OrderStatus: "未付款"},
	}
}

//具体的方法实现
func (os *OrderServiceImpl) GetOrderInfo(ctx context.Context, request *OrderRequest) (*OrderInfo, error) {
	startTime := time.Now()
	lock.RLock()
	defer lock.RUnlock()

	fmt.Printf("call GetOrderInfo ... %v", time.Now())

	//return nil, grpc.Errorf(codes.DataLoss, "data loss")

	response := &OrderInfo{}
	current := time.Now().Unix()
	if request.TimeStamp > current {
		response = &OrderInfo{OrderId: "0", OrderName: "", OrderStatus: "订单信息异常"}
	} else {
		result := orderMap[request.OrderId]
		if result.OrderId != "" {
			return &result, nil
		} else {
			return nil, errors.New("server error")
		}
	}

	fmt.Printf("call GetOrderInfo tc: %v\n", time.Since(startTime))

	return response, nil
}

//获取订单信息s
func (os *OrderServiceImpl) GetOrderInfos(request *OrderRequest, stream OrderService_GetOrderInfosServer) error {

	startTime := time.Now()
	lock.RLock()
	defer lock.RUnlock()

	for _, info := range orderMap {
		if time.Now().Unix() >= request.TimeStamp {
			//fmt.Println("订单序列号ID：", id)
			//fmt.Println("订单详情：", info)
			//通过流模式发送给客户端
			stream.Send(&info)
		}
	}

	fmt.Printf("call GetOrderInfos-Stream tc: %v\n", time.Since(startTime))
	return nil
}
