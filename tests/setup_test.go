package tests_test

import (
	"context"
	"os"
	"testing"

	"tickets/adapters"
	"tickets/message"
	"tickets/service"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

type TestFixtures struct {
	DB              *sqlx.DB
	RedisClient     *redis.Client
	SpreadsheetsAPI *adapters.SpreadsheetsAPIStub
	ReceiptsService *adapters.ReceiptsServiceStub
	PaymentsService *adapters.PaymentsServiceStub
	FileAPI         *adapters.FilesAPIStub
	DeadNationStub  *adapters.DeadNationStub
	Cancel          context.CancelFunc
}

func SetupComponentTest(t *testing.T) *TestFixtures {
	db, err := sqlx.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		panic(err)
	}

	rdb := message.NewRedisClient(os.Getenv("REDIS_ADDR"))

	ctx, cancel := context.WithCancel(context.Background())

	spreadsheetsAPI := adapters.NewSpreadsheetsAPIStub()
	receiptsService := adapters.NewReceiptsServiceStub()
	paymentsService := adapters.NewPaymentsServiceStub()
	fileAPI := adapters.NewFilesAPIStub()
	deadNationStub := adapters.NewDeadNationStub()

	go func() {
		svc, err := service.New(db, rdb, spreadsheetsAPI, receiptsService, paymentsService, fileAPI, deadNationStub)
		if err != nil {
			t.Errorf("failed to create service: %v", err)
			return
		}
		err = svc.Run(ctx)
		assert.NoError(t, err)
	}()

	waitForHttpServer(t)

	return &TestFixtures{
		DB:              db,
		RedisClient:     rdb,
		SpreadsheetsAPI: spreadsheetsAPI,
		ReceiptsService: receiptsService,
		PaymentsService: paymentsService,
		FileAPI:         fileAPI,
		DeadNationStub:  deadNationStub,
		Cancel:          cancel,
	}
}
