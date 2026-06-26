// Package config memuat konfigurasi aplikasi dari environment variable.
// Saat development, file `.env` dimuat otomatis via godotenv; di produksi env
// di-set langsung oleh orchestrator (tidak perlu file .env).
//
// Layer: infrastructure (frameworks & drivers) — paling luar di Clean Architecture.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config adalah root konfigurasi aplikasi.
type Config struct {
	App           AppConfig
	Server        ServerConfig
	Database      DatabaseConfig
	Security      SecurityConfig
	DOKU          DOKUConfig
	Observability ObservabilityConfig
}

type AppConfig struct {
	Name string
	Env  string // development | production
}

type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DSN membangun connection string PostgreSQL untuk GORM/pgx.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

type SecurityConfig struct {
	// MasterKey: kunci AES-GCM untuk enkripsi secret at-rest (detailed-design §9).
	MasterKey string
}

type DOKUConfig struct {
	Environment string // sandbox | production
	BaseURL     string
	ClientID    string
	SecretKey   string
}

// ObservabilityConfig menyetel observability (metrik Prometheus, dsb).
type ObservabilityConfig struct {
	// MetricsAddr adalah alamat listen endpoint /metrics pada worker (background
	// job). API server mengekspos /metrics pada port HTTP utamanya, sehingga
	// alamat ini khusus dipakai worker yang tidak punya HTTP server sendiri.
	MetricsAddr string
}

// Load membaca konfigurasi dari environment variable. File `.env` dimuat
// otomatis bila ada (tidak error bila tidak ada — mis. di produksi).
func Load() (*Config, error) {
	_ = godotenv.Load() // best-effort; abaikan bila .env tidak ada

	cfg := &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "payment-service"),
			Env:  getEnv("APP_ENV", "development"),
		},
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DATABASE_HOST", "localhost"),
			Port:            getEnvInt("DATABASE_PORT", 5432),
			User:            getEnv("DATABASE_USER", "postgres"),
			Password:        getEnv("DATABASE_PASSWORD", "postgres"),
			Name:            getEnv("DATABASE_NAME", "payment_service"),
			SSLMode:         getEnv("DATABASE_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DATABASE_CONN_MAX_LIFETIME", time.Hour),
		},
		Security: SecurityConfig{
			MasterKey: getEnv("PAYMENTS_MASTER_KEY", ""),
		},
		DOKU: DOKUConfig{
			Environment: getEnv("DOKU_ENVIRONMENT", "sandbox"),
			BaseURL:     getEnv("DOKU_BASE_URL", "https://api-sandbox.doku.com"),
			ClientID:    getEnv("DOKU_CLIENT_ID", ""),
			SecretKey:   getEnv("DOKU_SECRET_KEY", ""),
		},
		Observability: ObservabilityConfig{
			MetricsAddr: getEnv("METRICS_ADDR", ":9091"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Server.Port == 0 {
		return fmt.Errorf("config: SERVER_PORT tidak valid")
	}
	if c.Database.Host == "" || c.Database.Name == "" {
		return fmt.Errorf("config: DATABASE_HOST & DATABASE_NAME wajib diisi")
	}
	if c.App.Env == "production" {
		if c.Security.MasterKey == "" {
			return fmt.Errorf("config: PAYMENTS_MASTER_KEY wajib diisi di production")
		}
		// 32 byte = 64 hex char; panjang ganjil pasti bukan hex valid.
		if len(c.Security.MasterKey) != 64 {
			return fmt.Errorf("config: PAYMENTS_MASTER_KEY harus 64 karakter hex (32 byte), dapat %d karakter", len(c.Security.MasterKey))
		}
	}
	return nil
}

// --- helper pembaca env ---

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
