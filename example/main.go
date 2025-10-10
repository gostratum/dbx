package main

import (
	"embed"

	"github.com/gin-gonic/gin"
	"github.com/gostratum/core"
	"github.com/gostratum/dbx"
	"github.com/gostratum/httpx"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// User represents a user model
type User struct {
	ID    uint   `json:"id" gorm:"primaryKey"`
	Name  string `json:"name"`
	Email string `json:"email" gorm:"uniqueIndex"`
}

// Product represents a product model
type Product struct {
	ID    uint    `json:"id" gorm:"primaryKey"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func main() {
	app := core.New(
		// Database module with auto-migration and embedded SQL migrations
		dbx.Module(
			dbx.WithDefault("primary"),
			dbx.WithAutoMigrate(&User{}, &Product{}),
			dbx.WithMigrationsFS(migrationsFS, "migrations"),
			dbx.WithRunMigrations(),
			dbx.WithHealthChecks(),
		),
		// HTTP server module
		httpx.Module(
			httpx.WithBasePath("/api"),
		),
		// Register API routes
		fx.Invoke(registerRoutes),
	)

	// Run the application
	app.Run()
}

// registerRoutes registers the API routes
func registerRoutes(e *gin.Engine, db *gorm.DB, provider *dbx.Provider) {
	api := e.Group("/api")
	
	// Health endpoint
	api.GET("/health", func(c *gin.Context) {
		var result int
		if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
			c.JSON(500, gin.H{"error": "Database connection failed", "details": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "healthy", "database": result})
	})

	// User endpoints
	users := api.Group("/users")
	{
		users.GET("/", listUsers)
		users.POST("/", createUser)
		users.GET("/:id", getUser)
		users.PUT("/:id", updateUser)
		users.DELETE("/:id", deleteUser)
	}

	// Product endpoints  
	products := api.Group("/products")
	{
		products.GET("/", listProducts)
		products.POST("/", createProduct)
		products.GET("/:id", getProduct)
		products.PUT("/:id", updateProduct)
		products.DELETE("/:id", deleteProduct)
	}
}

// User handlers
func listUsers(c *gin.Context) {
	db := getDBFromContext(c)
	var users []User
	
	if err := db.Find(&users).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, users)
}

func createUser(c *gin.Context) {
	db := getDBFromContext(c)
	var user User
	
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	if err := db.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(201, user)
}

func getUser(c *gin.Context) {
	db := getDBFromContext(c)
	var user User
	id := c.Param("id")
	
	if err := db.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{"error": "User not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, user)
}

func updateUser(c *gin.Context) {
	db := getDBFromContext(c)
	var user User
	id := c.Param("id")
	
	if err := db.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{"error": "User not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	if err := db.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, user)
}

func deleteUser(c *gin.Context) {
	db := getDBFromContext(c)
	id := c.Param("id")
	
	if err := db.Delete(&User{}, id).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(204, nil)
}

// Product handlers
func listProducts(c *gin.Context) {
	db := getDBFromContext(c)
	var products []Product
	
	if err := db.Find(&products).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, products)
}

func createProduct(c *gin.Context) {
	db := getDBFromContext(c)
	var product Product
	
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	if err := db.Create(&product).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(201, product)
}

func getProduct(c *gin.Context) {
	db := getDBFromContext(c)
	var product Product
	id := c.Param("id")
	
	if err := db.First(&product, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, product)
}

func updateProduct(c *gin.Context) {
	db := getDBFromContext(c)
	var product Product
	id := c.Param("id")
	
	if err := db.First(&product, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{"error": "Product not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	if err := db.Save(&product).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, product)
}

func deleteProduct(c *gin.Context) {
	db := getDBFromContext(c)
	id := c.Param("id")
	
	if err := db.Delete(&Product{}, id).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(204, nil)
}

// getDBFromContext retrieves the database connection from the Gin context
func getDBFromContext(c *gin.Context) *gorm.DB {
	// In a real application, you might inject the DB into the context
	// For this example, we'll assume it's available in the context
	if db, exists := c.Get("db"); exists {
		return db.(*gorm.DB)
	}
	// This should not happen in a properly configured app
	panic("Database not found in context")
}

// Example using transaction helper
func transferMoney(c *gin.Context) {
	db := getDBFromContext(c)
	
	// Using the transaction helper
	err := dbx.WithTx(db, func(tx *gorm.DB) error {
		// Perform operations within the transaction
		var user1, user2 User
		if err := tx.First(&user1, 1).Error; err != nil {
			return err
		}
		if err := tx.First(&user2, 2).Error; err != nil {
			return err
		}
		
		// Business logic here...
		
		return nil
	})
	
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, gin.H{"status": "transfer completed"})
}