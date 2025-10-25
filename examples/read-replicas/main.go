package main

import (
	"context"
	"time"

	"github.com/gostratum/core"
	"github.com/gostratum/core/logx"
	"github.com/gostratum/dbx"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// User model for demonstration
type User struct {
	ID        uint   `gorm:"primarykey"`
	Name      string `gorm:"not null"`
	Email     string `gorm:"uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func main() {
	app := core.New(
		// Database module with auto-migration
		dbx.Module(
			dbx.WithDefault("primary"),
			dbx.WithAutoMigrate(&User{}),
			dbx.WithRunMigrations(),
			dbx.WithHealthChecks(),
		),

		// Demo functions
		fx.Invoke(runDemo),
	)

	app.Run()
}

func runDemo(lc fx.Lifecycle, db *gorm.DB, logger logx.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("=== DBX Read Replica Demo ===")

			// 1. Write to primary database
			logger.Info("1. Creating user (writes go to primary)")
			user := User{
				Name:  "Alice",
				Email: "alice@example.com",
			}

			// This write operation automatically goes to primary
			result := db.Create(&user)
			if result.Error != nil {
				logger.Error("Failed to create user", logx.Err(result.Error))
				return result.Error
			}
			logger.Info("User created successfully",
				logx.Int("id", int(user.ID)),
				logx.String("name", user.Name),
			)

			// 2. Read from replicas (default behavior for SELECT)
			logger.Info("2. Reading user (reads can go to replicas)")
			var fetchedUser User

			// By default, SELECT queries will use read replicas if configured
			result = db.First(&fetchedUser, user.ID)
			if result.Error != nil {
				logger.Error("Failed to fetch user", logx.Err(result.Error))
				return result.Error
			}
			logger.Info("User fetched from replica",
				logx.Int("id", int(fetchedUser.ID)),
				logx.String("name", fetchedUser.Name),
			)

			// 3. Force read from primary (useful after writes for consistency)
			logger.Info("3. Reading from primary explicitly")
			var primaryUser User

			// Use dbresolver.Write clause to force primary database
			result = dbx.WithPrimary(db).First(&primaryUser, user.ID)
			if result.Error != nil {
				logger.Error("Failed to fetch user from primary", logx.Err(result.Error))
				return result.Error
			}
			logger.Info("User fetched from primary",
				logx.Int("id", int(primaryUser.ID)),
				logx.String("name", primaryUser.Name),
			)

			// 4. Force read from replicas explicitly
			logger.Info("4. Reading from replicas explicitly")
			var replicaUser User

			// Use dbresolver.Read clause to force replicas
			result = dbx.WithReadReplicas(db).First(&replicaUser, user.ID)
			if result.Error != nil {
				logger.Error("Failed to fetch user from replica", logx.Err(result.Error))
				return result.Error
			}
			logger.Info("User fetched from replica",
				logx.Int("id", int(replicaUser.ID)),
				logx.String("name", replicaUser.Name),
			)

			// 5. List all users (uses replicas by default)
			logger.Info("5. Listing all users")
			var users []User
			result = db.Find(&users)
			if result.Error != nil {
				logger.Error("Failed to list users", logx.Err(result.Error))
				return result.Error
			}
			logger.Info("Users listed", logx.Int("count", len(users)))

			logger.Info("=== Demo Complete ===")
			logger.Info("Configuration: Check configs/base.yaml for read_replicas setup")
			logger.Info("Behavior:")
			logger.Info("  - Writes (INSERT, UPDATE, DELETE) always go to primary")
			logger.Info("  - Reads (SELECT) can use replicas by default")
			logger.Info("  - Use dbx.WithPrimary() to force primary for reads")
			logger.Info("  - Use dbx.WithReadReplicas() to explicitly use replicas")

			return nil
		},
	})
}
