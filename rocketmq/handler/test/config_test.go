package handler_test

import (
	"go-framework-demo/rocketmq"
	"go-framework-demo/rocketmq/handler"
	"go-framework-demo/util"
	"testing"
)

// TestConsumerTopicsAreSeparated 验证业务消息和 CDC 消息分别使用独立 Topic 及消费 Handler。
func TestConsumerTopicsAreSeparated(t *testing.T) {
	for group, callback := range handler.Handlers() {
		rocketmq.RegisterConsumerGroupHandler(group, callback)
	}
	config, err := util.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := rocketmq.LoadRuntimeConfig(config.RocketMQ); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"order_consumer_group":       "order_topic",
		"order_es_consumer_group":    "order_cdc_topic",
		"user_consumer_group_info":   "user_topic",
		"user_consumer_group_notify": "user_topic",
		"user_es_consumer_group":     "user_cdc_topic",
		"product_es_consumer_group":  "product_cdc_topic",
	}
	for _, config := range rocketmq.GlobalConsumerConfigs {
		topic, ok := expected[config.ConsumerGroup]
		if !ok {
			continue
		}
		if config.Topic != topic {
			t.Fatalf("%s consumes %s instead of %s", config.ConsumerGroup, config.Topic, topic)
		}
		delete(expected, config.ConsumerGroup)
	}
	if len(expected) != 0 {
		t.Fatalf("missing consumers: %v", expected)
	}
}
