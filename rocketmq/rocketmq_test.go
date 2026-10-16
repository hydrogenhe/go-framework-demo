package rocketmq

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/admin"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-framework-demo/util"

	"github.com/apache/rocketmq-client-go/v2/consumer"
)

var (
	broadcastMessages = make(chan *primitive.MessageExt, 32)
	clusterMessages   = make(chan *primitive.MessageExt, 32)
	delayMessages     = make(chan *primitive.MessageExt, 32)
)

// TestMain 注册测试消费回调并初始化真实 RocketMQ，测试结束后关闭生产者和消费者。
func TestMain(m *testing.M) {
	config, err := util.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "测试配置加载失败: %v\n", err)
		os.Exit(1)
	}
	if err := ensureRocketMQTestTopics(config.RocketMQ.Common.NameSrvAddr, config.RocketMQ.Common.BrokerAddr); err != nil {
		fmt.Fprintf(os.Stderr, "RocketMQ 测试 Topic 准备失败: %v\n", err)
		os.Exit(1)
	}
	// 测试使用专用 Topic 和消费者组，避免与应用实例争抢消息。
	testBusinesses := make([]util.RocketMQBusinessConfig, 0, 2)
	for _, business := range config.RocketMQ.Businesses {
		switch business.BizName {
		case "user":
			business.Topic = "go_framework_demo_test_user"
			business.Consumers = testConsumers(business.Consumers, "user_consumer_group_info", "user_consumer_group_notify")
			if len(business.Consumers) == 2 {
				RegisterConsumerGroupHandler(business.Consumers[0].ConsumerGroup, handlerFor(broadcastMessages))
				RegisterConsumerGroupHandler(business.Consumers[1].ConsumerGroup, handlerFor(clusterMessages))
				testBusinesses = append(testBusinesses, business)
			}
		case "order":
			business.Topic = "go_framework_demo_test_order"
			business.Consumers = testConsumers(business.Consumers, "order_consumer_group")
			if len(business.Consumers) == 1 {
				RegisterConsumerGroupHandler(business.Consumers[0].ConsumerGroup, handlerFor(delayMessages))
				testBusinesses = append(testBusinesses, business)
			}
		}
	}
	config.RocketMQ.Businesses = testBusinesses
	if err := InitAllRocketmq(config.RocketMQ); err != nil {
		fmt.Fprintf(os.Stderr, "RocketMQ 初始化失败: %v\n", err)
		_ = ShutdownAllRocketmq()
		os.Exit(1)
	}
	code := m.Run()
	_ = ShutdownAllRocketmq()
	os.Exit(code)
}

func ensureRocketMQTestTopics(nameServerAddr, brokerAddr string) error {
	client, err := admin.NewAdmin(admin.WithResolver(primitive.NewPassthroughResolver([]string{nameServerAddr})))
	if err != nil {
		return err
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	topics, err := client.FetchAllTopicList(ctx)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(topics.TopicList))
	for _, topic := range topics.TopicList {
		existing[topic] = struct{}{}
	}
	for _, topic := range []string{"go_framework_demo_test_user", "go_framework_demo_test_order"} {
		if _, ok := existing[topic]; ok {
			continue
		}
		if err := client.CreateTopic(ctx,
			admin.WithTopicCreate(topic),
			admin.WithBrokerAddrCreate(brokerAddr),
			admin.WithReadQueueNums(4),
			admin.WithWriteQueueNums(4),
			admin.WithPerm(6),
		); err != nil {
			return fmt.Errorf("创建 Topic %s: %w", topic, err)
		}
	}
	return nil
}

func testConsumers(consumers []util.RocketMQConsumerConfig, groups ...string) []util.RocketMQConsumerConfig {
	result := make([]util.RocketMQConsumerConfig, 0, len(groups))
	for _, group := range groups {
		for _, configured := range consumers {
			if configured.ConsumerGroup == group {
				configured.ConsumerGroup = strings.ReplaceAll(group, "_consumer_group", "") + "_test"
				result = append(result, configured)
				break
			}
		}
	}
	return result
}

// handlerFor 创建消费回调，将收到的消息放入通道，供测试检查消息内容。
func handlerFor(ch chan *primitive.MessageExt) ConsumeHandlerFunc {
	return func(_ context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, msg := range msgs {
			select {
			case ch <- msg:
			default:
			}
		}
		return consumer.ConsumeSuccess, nil
	}
}

// waitMessage 等待指定消息 ID 出现在通道中；超时则让测试失败。
func waitMessage(t *testing.T, ch chan *primitive.MessageExt, msgID string, timeout time.Duration) *primitive.MessageExt {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case msg := <-ch:
			if msg.MsgId == msgID {
				return msg
			}
		case <-timer.C:
			t.Fatalf("等待消息 %s 超时", msgID)
		}
	}
}

// TestRocketMQBroadcastConsume 发送测试消息，验证信息消费者收到的消息 ID 和内容；消费模式取决于当前配置。
func TestRocketMQBroadcastConsume(t *testing.T) {
	result, err := SendMsg(context.Background(), "user", []byte("broadcast-test"))
	if err != nil {
		t.Fatalf("发送广播测试消息失败: %v", err)
	}
	if msg := waitMessage(t, broadcastMessages, result.MsgID, 30*time.Second); string(msg.Body) != "broadcast-test" {
		t.Fatalf("广播消息内容不一致: %s", msg.Body)
	}
}

// TestRocketMQClusterConsume 发送测试消息，验证通知消费者收到的消息 ID 和内容。
func TestRocketMQClusterConsume(t *testing.T) {
	result, err := SendMsg(context.Background(), "user", []byte("cluster-test"))
	if err != nil {
		t.Fatalf("发送集群测试消息失败: %v", err)
	}
	if msg := waitMessage(t, clusterMessages, result.MsgID, 30*time.Second); string(msg.Body) != "cluster-test" {
		t.Fatalf("集群消息内容不一致: %s", msg.Body)
	}
}

// TestRocketMQDelayConsume 验证延时消息能够消费，且不会早于预期延时到达。
func TestRocketMQDelayConsume(t *testing.T) {
	msg := primitive.NewMessage(producerMap["order"].Config.Topic, []byte("delay-test"))
	msg.WithDelayTimeLevel(RocketMQDelayLevel2)
	sentAt := time.Now()
	result, err := SendDelayMsg(context.Background(), "order", msg)
	if err != nil {
		t.Fatalf("发送延时测试消息失败: %v", err)
	}
	received := waitMessage(t, delayMessages, result.MsgID, 60*time.Second)
	if string(received.Body) != "delay-test" {
		t.Fatalf("延时消息内容不一致: %s", received.Body)
	}
	if delay := time.Since(sentAt); delay < 4*time.Second {
		t.Fatalf("延时消息过早到达: %v", delay)
	}
}

// TestRocketMQOrderlyConsume 发送相同分片键的消息，检查收到的队列偏移量按发送顺序递增。
func TestRocketMQOrderlyConsume(t *testing.T) {
	// count 顺序消息测试发送的消息条数。
	const count = 3
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		msg := primitive.NewMessage(producerMap["order"].Config.Topic, []byte(fmt.Sprintf("orderly-test-%d", i)))
		msg.WithShardingKey("orderly-test-key")
		result, err := SendDelayMsg(context.Background(), "order", msg)
		if err != nil {
			t.Fatalf("发送顺序消息失败: %v", err)
		}
		ids = append(ids, result.MsgID)
	}
	var offsets []int64
	for _, id := range ids {
		msg := waitMessage(t, delayMessages, id, 30*time.Second)
		offsets = append(offsets, msg.QueueOffset)
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] <= offsets[i-1] {
			t.Fatalf("消息未按队列顺序消费: offsets=%v", offsets)
		}
	}
}
