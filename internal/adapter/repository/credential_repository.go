package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	appmw "github.com/Pravasta/payment-service/internal/adapter/http/middleware"
	"github.com/Pravasta/payment-service/internal/adapter/repository/model"
)

// CredentialRepository mengimplementasikan middleware.AuthRepository dengan GORM.
type CredentialRepository struct {
	db *gorm.DB
}

func NewCredentialRepository(db *gorm.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

// pastikan implementasi memenuhi kontrak port.
var _ appmw.AuthRepository = (*CredentialRepository)(nil)

// FindCredentialByKeyID mencari api_credential berdasarkan key_id publik.
// Mengembalikan error bila tidak ditemukan atau ada masalah DB.
func (r *CredentialRepository) FindCredentialByKeyID(ctx context.Context, keyID string) (*appmw.Credential, error) {
	var m model.APICredential
	err := r.db.WithContext(ctx).
		Where("key_id = ?", keyID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("credential not found")
	}
	if err != nil {
		return nil, err
	}
	return &appmw.Credential{
		ID:               m.ID,
		MerchantID:       m.MerchantID,
		SecretHash:       m.SecretHash,
		SigningSecretEnc: m.SigningSecretEnc,
		Scopes:           m.Scopes,
		Status:           m.Status,
	}, nil
}
