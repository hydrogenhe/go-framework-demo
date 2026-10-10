package rocketmq

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// 定义Topic类型枚举（替代硬编码）
type TopicType string

const (
	TopicUserInfoChange TopicType = "user_info_change" // 广播模式
	TopicUserNotifyPush TopicType = "user_notify_push" // 集群模式
)

// 测试用的临时结构体（替代全局变量，每个测试独立）
type testContext struct {
	msgChan       chan *primitive.MessageExt // 接收消费的消息
	expectedCount int                        // 预期收到消息的实例数
	receivedCount int                        // 实际收到的实例数
	mu            sync.Mutex
	doneChan      chan bool
	testMsgID     *string
	topicType     TopicType
	sendTime      time.Time // 消息发送时间（验证延时）
}

// 延时级别常量（复用原有逻辑，2级=5秒延时）
const (
	DelayLevel2        = 2                            // RocketMQ固定级别：5秒延时
	TopicOrderDelay    = "order_topic"                // 延时消息使用order_topic
	ConsumerGroupOrder = "order_delay_consumer_group" // 订单延时消息消费者组（集群模式）
)

// 延时消息测试上下文（复用原有风格）

// -------------------------- 回调注册移到外部（独立函数） --------------------------
// registerGeneralConsumeHandler 通用消费回调注册：外置独立函数
func registerGeneralConsumeHandler(tc *testContext, topicType TopicType) {
	handler := func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		instanceID := extractInstanceIdFromCtx(ctx)
		clientID := extractClientIdFromCtx(ctx)

		for _, msg := range msgs {
			// 仅处理测试消息
			if tc.testMsgID == nil || *tc.testMsgID == "" {
				return consumer.ConsumeSuccess, nil
			}
			if msg.MsgId != *tc.testMsgID {
				continue
			}

			// 1. 按Topic类型打印
			switch topicType {
			case TopicUserInfoChange: // 广播模式
				log.Printf("[TEST-广播-实例%d,client[%v]] 收到原始消息：MsgID=%s, Body=%s", instanceID, clientID, msg.MsgId, string(msg.Body))
			case TopicUserNotifyPush: // 集群模式
				log.Printf("[TEST-集群-实例%d,clientID[%v]] 收到原始消息：MsgID=%s, Body=%s", instanceID, clientID, msg.MsgId, string(msg.Body))
			}

			// 2. 解析UserMessage并打印结构化内容
			userMsg, err := ParseUserMessage(string(msg.Body))
			if err != nil {
				log.Printf("[TEST-%s-实例%d] 反序列化UserMessage失败：%v", topicType, instanceID, err)
				return consumer.ConsumeSuccess, nil
			}
			log.Printf("[TEST-%s-实例%d] 解析后的UserMessage：%+v", topicType, instanceID, userMsg)

			// 3. 发送到管道（供验证）
			tc.msgChan <- msg

			// 4. 统计收到的实例数
			tc.mu.Lock()
			tc.receivedCount++
			receivedCount := tc.receivedCount
			tc.mu.Unlock()

			// 达到预期数量触发完成信号
			if receivedCount >= tc.expectedCount {
				tc.doneChan <- true
			}
		}
		return consumer.ConsumeSuccess, nil
	}

	// 注册到对应Topic
	switch topicType {
	case TopicUserInfoChange:
		RegisterConsumeHandler(string(TopicUserInfoChange), handler)
	case TopicUserNotifyPush:
		RegisterConsumeHandler(string(TopicUserNotifyPush), handler)
	}
}

// TestRocketMQUserBusiness 最终版：回调注册外置 + 优雅退出/验证内联 + 无全局变量
func TestRocketMQUserBusiness(t *testing.T) {
	// 1. 优雅退出逻辑（移到测试函数内）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		t.Log("=== 接收到退出信号，优雅关闭RocketMQ ===")
		ShutdownAllRocketmq()
		os.Exit(0)
	}()

	// 2. 初始化日志
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx := context.Background()

	// 3. 测试结束后优雅关闭
	defer func() {
		t.Log("=== 开始关闭RocketMQ ===")
		if err := ShutdownAllRocketmq(); err != nil {
			t.Errorf("RocketMQ关闭失败: %v", err)
		}
	}()
	var testMsgID string // 声明变量，后续赋值

	broadcastCtx := &testContext{
		msgChan:       make(chan *primitive.MessageExt, 1),
		expectedCount: 1, // 单实例，预期1个
		doneChan:      make(chan bool, 1),
		testMsgID:     &testMsgID,
		topicType:     TopicUserInfoChange,
	}
	clusterCtx := &testContext{
		msgChan:       make(chan *primitive.MessageExt, 1),
		expectedCount: 1, // 单实例，预期1个
		doneChan:      make(chan bool, 1),
		testMsgID:     &testMsgID,
		topicType:     TopicUserNotifyPush,
	}
	// 调用外部的回调注册函数
	registerGeneralConsumeHandler(broadcastCtx, TopicUserInfoChange)
	registerGeneralConsumeHandler(clusterCtx, TopicUserNotifyPush)

	// 4. 初始化RocketMQ（加载配置，单实例模式）
	t.Log("=== 开始初始化RocketMQ（单实例模式）===")
	if err := InitAllRocketmq("../conf/rocketmq.yaml"); err != nil {
		t.Fatalf("RocketMQ初始化失败: %v", err)
	}

	// 5. 等待消费者启动
	t.Log("=== 等待3秒，确保消费者实例启动完成 ===")
	time.Sleep(3 * time.Second)

	// 6. 构造UserMessage并发送
	t.Log("=== 构造并发送UserMessage测试消息 ===")
	userMsg, err := NewUserMessage(ActionUpdate, "1001", "测试用户")
	if err != nil {
		t.Fatalf("创建UserMessage失败: %v", err)
	}
	userMsg.Phone = "13800138000"
	userMsg.Email = "test@example.com"
	userMsg.ExtInfo = `{"test":"true"}`

	msgJSON, err := userMsg.ToJSON()
	if err != nil {
		t.Fatalf("UserMessage序列化为JSON失败: %v", err)
	}
	t.Logf("构造的UserMessage：%+v", userMsg)

	// 发送消息
	sendResult, err := UserProducerSendMsg(ctx, []byte(msgJSON))
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	testMsgID = sendResult.MsgID
	t.Logf("测试消息发送成功，MsgID: %s，JSON内容：%s", testMsgID, msgJSON)

	// -------------------------- 验证逻辑内联（替代外部函数） --------------------------
	validate := func(tc *testContext, mode string) {
		timeout := time.After(30 * time.Second)
		select {
		case <-tc.doneChan:
			tc.mu.Lock()
			t.Logf("✅ %s模式验证通过：收到消息的实例数=%d（预期=%d）", mode, tc.receivedCount, tc.expectedCount)
			tc.mu.Unlock()
		case <-timeout:
			t.Errorf("❌ %s模式验证超时（15秒）", mode)
		}
		// 关闭管道（避免泄漏）
		close(tc.msgChan)
		close(tc.doneChan)
	}

	// 7. 验证广播模式（单实例，预期1个实例收到）

	// 8. 验证集群模式（单实例，预期1个实例收到）
	t.Log("\n=== 验证集群模式（clustering）- 单实例预期1个收到 ===")

	// 调用外部的回调注册函数
	validate(broadcastCtx, "广播")
	validate(clusterCtx, "集群")

	// 9. 测试总结
	t.Log("\n=== 测试总结 ===")
	t.Log("📢 广播模式（broadcasting）：单实例收到全量消息")
	t.Log("📢 集群模式（clustering）：单实例独占所有消息（多实例时会分摊）")
	time.Sleep(3 * time.Second)
}

// -------------------------- 原有辅助函数保留 --------------------------
func extractInstanceIdFromCtx(ctx context.Context) int {
	if instanceId, ok := ctx.Value(InstanceIDKey).(int); ok {
		return instanceId
	}
	return 0
}

func extractClientIdFromCtx(ctx context.Context) string {
	if clientId, ok := ctx.Value(ClientIdKey).(string); ok {
		return clientId
	}
	return "unknown-client"
}

// registerOrderDelayConsumeHandler 注册订单延时消息消费回调（集群模式）
func registerOrderDelayConsumeHandler(dtc *testContext) {
	handler := func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		instanceID := extractInstanceIdFromCtx(ctx)
		clientID := extractClientIdFromCtx(ctx)

		for _, msg := range msgs {
			// 仅处理测试的订单延时消息
			if dtc.testMsgID == nil || *dtc.testMsgID == "" || msg.MsgId != *dtc.testMsgID {
				return consumer.ConsumeSuccess, nil
			}

			// 1. 打印基础信息
			log.Printf("[TEST-订单延时-集群-实例%d,client[%v]] 收到延时消息：MsgID=%s, Body=%s",
				instanceID, clientID, msg.MsgId, string(msg.Body))

			// 2. 计算实际延时（发送时间 vs 接收时间）
			receiveTime := time.Now()
			actualDelay := receiveTime.Sub(dtc.sendTime)
			log.Printf("[TEST-订单延时] 预期延时：5秒，实际延时：%.1f秒", actualDelay.Seconds())

			// 3. 解析OrderMessage订单消息体
			orderMsg, err := ParseOrderMessage(string(msg.Body))
			if err != nil {
				log.Printf("[TEST-订单延时-实例%d] 反序列化OrderMessage失败：%v", instanceID, err)
				return consumer.ConsumeSuccess, nil
			}
			log.Printf("[TEST-订单延时-实例%d] 解析后的订单消息：%+v", instanceID, orderMsg)

			// 4. 统计并触发完成信号
			dtc.mu.Lock()
			dtc.receivedCount++
			receivedCount := dtc.receivedCount
			dtc.mu.Unlock()

			dtc.msgChan <- msg
			if receivedCount >= 1 { // 集群模式仅1个实例收到
				dtc.doneChan <- true
			}
		}
		return consumer.ConsumeSuccess, nil
	}

	// 注册到订单延时消费者组（集群模式）
	RegisterConsumeHandler("order", handler)
}

// TestRocketMQOrderDelayMessage 订单延时消息自发自收测试（集群模式+order_topic）
func TestRocketMQOrderDelayMessage(t *testing.T) {
	// 1. 优雅退出逻辑
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		t.Log("=== 接收到退出信号，优雅关闭RocketMQ ===")
		ShutdownAllRocketmq()
		os.Exit(0)
	}()

	// 2. 初始化日志
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx := context.Background()

	// 3. 测试结束后优雅关闭
	defer func() {
		t.Log("=== 开始关闭RocketMQ ===")
		if err := ShutdownAllRocketmq(); err != nil {
			t.Errorf("RocketMQ关闭失败: %v", err)
		}
	}()

	// 4. 初始化延时消息测试上下文
	var testMsgID string
	dtc := &testContext{
		msgChan:   make(chan *primitive.MessageExt, 1),
		sendTime:  time.Time{},
		doneChan:  make(chan bool, 1),
		testMsgID: &testMsgID,
	}

	// 5. 注册订单延时消息消费回调（集群模式）
	registerOrderDelayConsumeHandler(dtc)

	// 6. 初始化RocketMQ（加载配置）
	t.Log("=== 开始初始化RocketMQ（订单延时消息测试）===")
	if err := InitAllRocketmq("../conf/rocketmq.yaml"); err != nil {
		t.Fatalf("RocketMQ初始化失败: %v", err)
	}

	// 7. 等待消费者启动
	t.Log("=== 等待3秒，确保订单延时消息消费者启动完成 ===")
	time.Sleep(3 * time.Second)

	// 8. 构造订单延时消息并发送（核心：基于OrderMessage）
	t.Log("=== 构造并发送5秒延时订单消息 ===")
	// 创建标准化OrderMessage
	orderMsg, err := NewOrderMessage(ActionCreate, "ORDER_10086", "USER_789", 99.9)
	if err != nil {
		t.Fatalf("创建OrderMessage失败: %v", err)
	}
	orderMsg.Status = "待支付"                           // 补充订单状态
	dtc.sendTime = time.UnixMilli(orderMsg.Timestamp) // 记录发送时间

	// 序列化为JSON
	msgJSON, err := orderMsg.ToJSON()
	if err != nil {
		t.Fatalf("OrderMessage序列化为JSON失败: %v", err)
	}
	t.Logf("构造的延时订单消息：%+v", orderMsg)

	// 构建RocketMQ延时消息（指定order_topic+5秒延时）
	rocketMsg := primitive.NewMessage(TopicOrderDelay, []byte(msgJSON))
	rocketMsg.WithDelayTimeLevel(RocketMQDelayLevel2) // 设置2级延时（5秒）

	// 发送延时消息（复用原有生产者发送逻辑）
	sendResult, err := SendDelayMsg(ctx, "order", rocketMsg) // 适配你原有SendMsg方法
	if err != nil {
		t.Fatalf("发送订单延时消息失败: %v", err)
	}
	testMsgID = sendResult.MsgID
	t.Logf("订单延时消息发送成功，MsgID: %s，预期5秒后消费", testMsgID)

	// 9. 验证延时消息接收（超时60秒）
	validateOrderDelay := func(dtc *testContext) {
		timeout := time.After(60 * time.Second)
		select {
		case <-dtc.doneChan:
			dtc.mu.Lock()
			t.Logf("✅ 订单延时消息验证通过：收到消息数=%d（预期=1）", dtc.receivedCount)
			dtc.mu.Unlock()
		case <-timeout:
			t.Errorf("❌ 订单延时消息消费超时（60秒）")
			t.Fatalf("❌ 订单延时消息消费超时（60秒）！断言失败：未在规定时间内收到延时消息")
		}
		// 关闭管道
		close(dtc.msgChan)
		close(dtc.doneChan)
	}

	// 10. 验证集群模式订单延时消息消费
	t.Log("\n=== 验证集群模式订单延时消息消费 ===")
	validateOrderDelay(dtc)

	// 11. 测试总结
	t.Log("\n=== 订单延时消息测试总结 ===")
	t.Log("📢 集群模式订单延时消息：基于order_topic发送5秒延时消息，仅1个实例收到并解析OrderMessage")
	time.Sleep(3 * time.Second)
}
