// reconcile-es 先正向补偿，再反向更新/删除 ES；不会修改 MySQL。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go-framework-demo/es"
	"go-framework-demo/logger"
	"go-framework-demo/mysql"
	"go-framework-demo/scheduler/reconciliation"
	"go-framework-demo/util"
)

func main() {
	logger.Init()
	err := run()
	logger.Sync()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	config, err := util.LoadConfig()
	if err != nil {
		return err
	}
	if err := mysql.InitMySQL(config.MySQL); err != nil {
		return err
	}
	if err := es.InitES(config.ES); err != nil {
		return err
	}
	return reconciliation.Run(ctx, mysql.GetDB())
}
