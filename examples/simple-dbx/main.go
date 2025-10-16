package main

import (
	"context"

	"github.com/gostratum/core"
	"github.com/gostratum/core/logx"
	"github.com/gostratum/dbx"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// User model for demonstration
type User struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Name  string `json:"name"`
	Email string `json:"email" gorm:"uniqueIndex"`
}

func main() {
	app := core.New(
		// DBX module with new configuration pattern
		dbx.Module(
			dbx.WithDefault("primary"),
			dbx.WithAutoMigrate(&User{}),
			dbx.WithHealthChecks(), // Enables health monitoring
		),

		// Application logic
		fx.Invoke(RunDemo),
	)

	app.Run()
}

// RunDemo demonstrates the new dependency injection pattern
func RunDemo(db *gorm.DB, logger logx.Logger, lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting simple DBX example")

			// Create some sample users
			users := []User{
				{Name: "Alice Johnson", Email: "alice@example.com"},
				{Name: "Bob Smith", Email: "bob@example.com"},
				{Name: "Carol Williams", Email: "carol@example.com"},
			}

			for _, user := range users {
				if err := db.WithContext(ctx).Create(&user).Error; err != nil {
					logger.Error("Failed to create user",
						logx.Err(err),
						logx.String("email", user.Email))
					return err
				}

				logger.Info("User created",
					logx.Int("id", int(user.ID)),
					logx.String("name", user.Name),
					logx.String("email", user.Email))
			}

			// Query all users
			var allUsers []User
			if err := db.WithContext(ctx).Find(&allUsers).Error; err != nil {
				logger.Error("Failed to query users", logx.Err(err))
				return err
			}

			logger.Info("Total users in database", logx.Int("count", len(allUsers)))

			// Demonstrate updating
			var alice User
			if err := db.WithContext(ctx).Where("email = ?", "alice@example.com").First(&alice).Error; err != nil {
				logger.Error("Failed to find Alice", logx.Err(err))
				return err
			}

			alice.Name = "Alice Johnson-Smith"
			if err := db.WithContext(ctx).Save(&alice).Error; err != nil {
				logger.Error("Failed to update Alice", logx.Err(err))
				return err
			}

			logger.Info("User updated",
				logx.Int("id", int(alice.ID)),
				logx.String("new_name", alice.Name))

			logger.Info("Simple DBX example completed successfully!")
			logger.Info("Press Ctrl+C to exit")

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down simple DBX example")
			return nil
		},
	})
}
