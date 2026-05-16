package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"whatsgo/pkg/logger"
	"github.com/joho/godotenv"
	"go.mau.fi/whatsmeow"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	call_handler "whatsgo/pkg/call/handler"
	call_service "whatsgo/pkg/call/service"
	chat_handler "whatsgo/pkg/chat/handler"
	chat_service "whatsgo/pkg/chat/service"
	community_handler "whatsgo/pkg/community/handler"
	community_service "whatsgo/pkg/community/service"
	config "whatsgo/pkg/config"
	producer_interfaces "whatsgo/pkg/events/interfaces"
	nats_producer "whatsgo/pkg/events/nats"
	webhook_producer "whatsgo/pkg/events/webhook"
	websocket_producer "whatsgo/pkg/events/websocket"
	group_handler "whatsgo/pkg/group/handler"
	group_service "whatsgo/pkg/group/service"
	instance_handler "whatsgo/pkg/instance/handler"
	instance_model "whatsgo/pkg/instance/model"
	instance_repository "whatsgo/pkg/instance/repository"
	instance_service "whatsgo/pkg/instance/service"
	label_handler "whatsgo/pkg/label/handler"
	label_model "whatsgo/pkg/label/model"
	label_repository "whatsgo/pkg/label/repository"
	label_service "whatsgo/pkg/label/service"
	logger_wrapper "whatsgo/pkg/logger"
	message_handler "whatsgo/pkg/message/handler"
	message_model "whatsgo/pkg/message/model"
	message_repository "whatsgo/pkg/message/repository"
	message_service "whatsgo/pkg/message/service"
	auth_middleware "whatsgo/pkg/middleware"
	newsletter_handler "whatsgo/pkg/newsletter/handler"
	newsletter_service "whatsgo/pkg/newsletter/service"
	routes "whatsgo/pkg/routes"
	send_handler "whatsgo/pkg/sendMessage/handler"
	send_service "whatsgo/pkg/sendMessage/service"
	server_handler "whatsgo/pkg/server/handler"
	storage_interfaces "whatsgo/pkg/storage/interfaces"
	minio_storage "whatsgo/pkg/storage/minio"
	user_handler "whatsgo/pkg/user/handler"
	user_service "whatsgo/pkg/user/service"
	whatsmeow_service "whatsgo/pkg/whatsmeow/service"
)

var devMode = flag.Bool("dev", false, "Enable development mode")

func setupRouter(db *gorm.DB, authDB *sql.DB, sqliteDB *sql.DB, config *config.Config, exPath string) *gin.Engine {
	killChannel := make(map[string](chan bool))
	clientPointer := make(map[string]*whatsmeow.Client)

	loggerWrapper := logger_wrapper.NewLoggerManager(config)

	var natsProducer producer_interfaces.Producer
	if config.NatsUrl != "" {
		logger.LogInfo("NATS enabled")
		natsProducer = nats_producer.NewNatsProducer(
			config.NatsUrl,
			config.NatsGlobalEnabled,
			config.NatsGlobalEvents,
			loggerWrapper,
		)
	} else {
		natsProducer = nats_producer.NewNatsProducer(
			"",
			false,
			nil,
			loggerWrapper,
		)
	}

	webhookProducer := webhook_producer.NewWebhookProducer(config.WebhookUrl, loggerWrapper)
	websocketProducer := websocket_producer.NewWebsocketProducer(loggerWrapper)



	var mediaStorage storage_interfaces.MediaStorage
	var err error
	if config.MinioEnabled {
		mediaStorage, err = minio_storage.NewMinioMediaStorage(
			config.MinioEndpoint,
			config.MinioAccessKey,
			config.MinioSecretKey,
			config.MinioBucket,
			config.MinioRegion,
			config.MinioUseSSL,
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	instanceRepository := instance_repository.NewInstanceRepository(db)
	messageRepository := message_repository.NewMessageRepository(db)
	labelRepository := label_repository.NewLabelRepository(db)

	whatsmeowService := whatsmeow_service.NewWhatsmeowService(
		instanceRepository,
		authDB,
		message_repository.NewMessageRepository(db),
		labelRepository,
		config,
		killChannel,
		clientPointer,
		webhookProducer,
		websocketProducer,
		sqliteDB,
		exPath,
		mediaStorage,
		natsProducer,
		loggerWrapper,
	)
	instanceService := instance_service.NewInstanceService(
		instanceRepository,
		killChannel,
		clientPointer,
		whatsmeowService,
		config,
		loggerWrapper,
	)
	sendMessageService := send_service.NewSendService(clientPointer, whatsmeowService, config, loggerWrapper)
	userService := user_service.NewUserService(clientPointer, whatsmeowService, loggerWrapper)
	messageService := message_service.NewMessageService(clientPointer, messageRepository, whatsmeowService, loggerWrapper)
	chatService := chat_service.NewChatService(clientPointer, whatsmeowService, loggerWrapper)
	groupService := group_service.NewGroupService(clientPointer, whatsmeowService, loggerWrapper)
	callService := call_service.NewCallService(clientPointer, whatsmeowService, loggerWrapper)
	communityService := community_service.NewCommunityService(clientPointer, whatsmeowService, loggerWrapper)
	labelService := label_service.NewLabelService(clientPointer, whatsmeowService, labelRepository, loggerWrapper)
	newsletterService := newsletter_service.NewNewsletterService(clientPointer, whatsmeowService, loggerWrapper)

	r := gin.Default()
	routes.NewRouter(
		auth_middleware.NewMiddleware(config, instanceService),
		instance_handler.NewInstanceHandler(instanceService, config),
		user_handler.NewUserHandler(userService),
		send_handler.NewSendHandler(sendMessageService),
		message_handler.NewMessageHandler(messageService),
		chat_handler.NewChatHandler(chatService),
		group_handler.NewGroupHandler(groupService),
		call_handler.NewCallHandler(callService),
		community_handler.NewCommunityHandler(communityService),
		label_handler.NewLabelHandler(labelService),
		newsletter_handler.NewNewsletterHandler(newsletterService),
		server_handler.NewServerHandler(),
	).AssignRoutes(r)

	if config.ConnectOnStartup {
		go whatsmeowService.ConnectOnStartup(config.ClientName)
	}

	r.GET("/ws", func(c *gin.Context) {
		token := c.Query("token")
		instanceId := c.Query("instanceId")

		if token != config.GlobalApiKey {
			logger.LogError("Token inválido: %s", token)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido"})
			return
		}

		websocket_producer.ServeWs(c.Writer, c.Request, instanceId, websocketProducer)
	})

	return r
}

func migrate(db *gorm.DB) {
	err := db.AutoMigrate(&instance_model.Instance{}, &message_model.Message{}, &label_model.Label{})

	if err != nil {
		log.Fatal(err)
	}
}

func initAuthDB(config *config.Config) (*sql.DB, string, error) {
	if config.PostgresAuthDB != "" {
		return nil, "", nil
	}

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	dbDirectory := exPath + "/dbdata"
	_, err = os.Stat(dbDirectory)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(dbDirectory, 0751)
		if errDir != nil {
			panic("Could not create dbdata directory")
		}
	}

	db, err := sql.Open("sqlite", exPath+"/dbdata/users.db?_pragma=foreign_keys(1)&_busy_timeout=3000")
	if err != nil {
		return nil, "", err
	}

	return db, exPath, nil
}

func initPostgresAuthDB(config *config.Config) (*sql.DB, error) {
	if config.PostgresAuthDB == "" {
		return nil, nil
	}

	db, err := sql.Open("postgres", config.PostgresAuthDB)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco AUTH PostgreSQL: %v", err)
	}

	// Configurar pool de conexões para evitar conexões ociosas não fechadas
	db.SetMaxOpenConns(25)                 // Máximo de 25 conexões abertas simultaneamente
	db.SetMaxIdleConns(5)                  // Máximo de 5 conexões ociosas no pool
	db.SetConnMaxLifetime(5 * time.Minute) // Reconectar após 5 minutos para evitar timeouts
	db.SetConnMaxIdleTime(1 * time.Minute) // Fechar conexões ociosas após 1 minuto

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("erro ao pingar banco AUTH PostgreSQL: %v", err)
	}

	logger.LogInfo("Conectado ao banco AUTH PostgreSQL com pool configurado")
	return db, nil
}

// @title Whatsgo
// @version 1.0
// @description Whatsgo - whatsmeow
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name apikey
func main() {
	flag.Parse()
	if *devMode {
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal(err)
		}
	}

	config := config.Load()

	licenseToken := config.GlobalApiKey
	if licenseToken == "" {
		log.Fatal("GlobalApiKey não configurado")
	}

	db, err := config.CreateUsersDB()

	if err != nil {
		log.Fatal(err)
	}

	// Inicializar PostgreSQL AUTH
	authDB, err := initPostgresAuthDB(config)
	if err != nil {
		log.Fatal(err)
	}
	if authDB != nil {
		defer authDB.Close()
	}

	// Manter inicialização do SQLite
	sqliteDB, exPath, err := initAuthDB(config)
	if err != nil {
		log.Fatal(err)
	}
	defer sqliteDB.Close()

	migrate(db)



	r := setupRouter(db, authDB, sqliteDB, config, exPath)

	logger.LogInfo("Iniciando servidor na porta %s", os.Getenv("SERVER_PORT"))

	srv := &http.Server{
		Addr:    ":" + os.Getenv("SERVER_PORT"),
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.LogError("listen: %s\n", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.LogInfo("Sinal de desligamento recebido. Desligando servidor (Graceful Shutdown)...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.LogError("Erro no Server Shutdown: %v", err)
	}

	logger.LogInfo("Servidor desligado com sucesso.")
}
