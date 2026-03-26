package pkg

import (
	"context"
	"log"
	"solution/config"
	"solution/handler"
	"solution/migration"
	"solution/repository"
	"solution/service"

	"github.com/gin-gonic/gin"
)

// запуск сервера
func Run_Server() {
	cfg := config.Load()

	// подключаем бд
	postgres, err := repository.NewPostgres(cfg.GetDatabaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer postgres.Close()

	log.Printf("Connecting to database: %s@%s:%s/%s",
		cfg.DBhost, cfg.DBname, cfg.DBport, cfg.DBuser)

	userRepo := repository.NewUserRepository(postgres.DB)
	fraudRepo := repository.NewFraudRepository(postgres.DB)

	DB := postgres.GetDB()
	if err := migration.CreateBase(DB); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	userService := service.NewUserService(userRepo, cfg.JWTsecret, cfg.AdminPassword, cfg.AdminEmail, cfg.AdminUsername)
	if err := userService.CreateInitialAdmin(ctx); err != nil {
		log.Fatal(err)
	}

	fraudService := service.NewFraudService(fraudRepo)
	transactionRepo := repository.NewTransactionRepository(postgres.DB)
	ruleRepo := repository.NewRuleRepository(postgres.DB)

	transactionService := service.NewTransactionServiceQQ(
		transactionRepo,
		userRepo,
		ruleRepo,
	)

	auth := handler.NewAuthHandler(userService, cfg.JWTsecret)
	userHandler := handler.NewHandler(userService, cfg.JWTsecret)
	fraudHandler := handler.NewHandlerFraud(fraudService)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	r := gin.Default()

	if gin.Mode() == gin.DebugMode {
		r.Use(func(c *gin.Context) {
			c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
			c.Next()
		})
	}

	r.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"HELLO": "channel subscribers Virus Sdutio"})
	})
	r.POST("/api/v1/auth/register", auth.Register)
	r.POST("/api/v1/auth/login", auth.Login)

	api := r.Group("/api/v1")
	api.Use(userHandler.ProvToken())
	{
		users := api.Group("/users")
		{
			users.GET("/me", userHandler.UsersMe)
			users.PUT("/me", userHandler.UpdateUSerID)

			users.GET("/:id", userHandler.UserID)
			users.PUT("/:id", userHandler.UpdateUSerID)

			adminRoutes := users.Group("")
			adminRoutes.Use(userHandler.AdminOnly())
			{
				adminRoutes.GET("", userHandler.AllUsers)
				adminRoutes.POST("", userHandler.CreateUser)
				adminRoutes.DELETE("/:id", userHandler.DeactivateUser)
			}
		}
	}

	fraudGroup := r.Group("/api/v1/fraud-rules")

	fraudGroup.Use(userHandler.ProvToken())
	{
		fraudGroup.POST("", fraudHandler.CreateFraud)
		fraudGroup.GET("", fraudHandler.AllFraud)
		fraudGroup.GET("/:id", fraudHandler.GetBYID)
		fraudGroup.PUT("/:id", fraudHandler.UpdateFraudID)
		fraudGroup.DELETE("/:id", fraudHandler.DeactivateFraudID)
		fraudGroup.POST("/validate", fraudHandler.ValiteDLS)
	}

	tractions := r.Group("/api/v1/transactions")

	tractions.Use(userHandler.ProvToken())
	{
		transactions := api.Group("/transactions")
		transactions.POST("", transactionHandler.CreateTransaction)
		transactions.GET("", transactionHandler.GetTransactions)
		transactions.GET("/:id", transactionHandler.GetTransaction)
	}

	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
