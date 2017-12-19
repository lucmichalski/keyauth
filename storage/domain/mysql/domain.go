package mysql

import (
	"database/sql"
	"strconv"
	"sync"
	"time"

	"github.com/satori/go.uuid"

	"openauth/api/exception"
	"openauth/storage/domain"
)

var (
	createPrepare *sql.Stmt
	deletePrepare *sql.Stmt
)

// NewDomainStorage use to create domain storage service
func NewDomainStorage(db *sql.DB) domain.Storage {
	return &manager{db: db}
}

// DomainManager is use mongodb as storage
type manager struct {
	db *sql.DB
}

// CreateDomain use to create an domain
func (m *manager) CreateDomain(name, description, displayName string, enabled bool) (*domain.Domain, error) {
	var (
		once sync.Once
		err  error
	)

	ok, err := m.CheckDomainIsExistByName(name)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, exception.NewBadRequest("domain %s exist", name)
	}

	once.Do(func() {
		createPrepare, err = m.db.Prepare("INSERT INTO `domain` (id, name, display_name, description, enabled, extra, create_at) VALUES (?,?,?,?,?,?,?)")
	})
	if err != nil {
		return nil, exception.NewInternalServerError("prepare insert domain stmt error, domain: %s, %s", name, err)
	}

	dom := domain.Domain{ID: uuid.NewV4().String(), Name: name, DisplayName: displayName, Description: description, CreateAt: time.Now().Unix(), Enabled: enabled}
	_, err = createPrepare.Exec(dom.ID, dom.Name, dom.DisplayName, dom.Description, dom.Enabled, "", dom.CreateAt)
	if err != nil {
		return nil, exception.NewInternalServerError("insert domain exec sql err, %s", err)
	}
	return &dom, nil
}

// GetDomain use to get domain detail
func (m *manager) GetDomain(domainID string) (*domain.Domain, error) {
	dom := domain.Domain{}
	err := m.db.QueryRow("SELECT id,name,display_name,description,enabled,create_at,update_at FROM domain WHERE id = ?", domainID).Scan(
		&dom.ID, &dom.Name, &dom.DisplayName, &dom.Description, &dom.Enabled, &dom.CreateAt, &dom.UpdateAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, exception.NewNotFound("domain %s not find", domainID)
		}

		return nil, exception.NewInternalServerError("query single domain error, %s", err)
	}

	return &dom, nil
}

func (m *manager) GetDomainByName(name string) (*domain.Domain, error) {
	dom := domain.Domain{}
	err := m.db.QueryRow("SELECT id,name,display_name,description,enabled,create_at,update_at FROM domain WHERE name = ?", name).Scan(
		&dom.ID, &dom.Name, &dom.DisplayName, &dom.Description, &dom.Enabled, &dom.CreateAt, &dom.UpdateAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, exception.NewNotFound("domain %s not find", name)
		}

		return nil, exception.NewInternalServerError("query single domain error, %s", err)
	}

	return &dom, nil
}

// ListDomain use to list all domains
func (m *manager) ListDomain(pageNumber, pageSize string) ([]*domain.Domain, int64, error) {
	var (
		rows   *sql.Rows
		err    error
		pn     int64
		ps     int64
		totalP int64
	)

	if pageNumber != "" {
		pn, err = strconv.ParseInt(pageNumber, 10, 64)
		if err != nil {
			return nil, 0, exception.NewBadRequest("page_size must be number")
		}
	}
	if pageSize != "" {
		ps, err = strconv.ParseInt(pageSize, 10, 64)
		if err != nil {
			return nil, 0, exception.NewBadRequest("page_number must be number")
		}
	}

	offset := (pn - 1) * ps
	limit := ps

	if ps != 0 {
		rows, err = m.db.Query("SELECT id,name,display_name,description,enabled,create_at,update_at FROM domain ORDER BY create_at DESC LIMIT ?,?", offset, limit)
	} else {
		rows, err = m.db.Query("SELECT id,name,display_name,description,enabled,create_at,update_at FROM domain ORDER BY create_at DESC")
	}
	if err != nil {
		return nil, 0, exception.NewInternalServerError("query domain list error, %s", err)
	}
	defer rows.Close()

	domains := []*domain.Domain{}
	for rows.Next() {
		dom := domain.Domain{}
		if err := rows.Scan(&dom.ID, &dom.Name, &dom.DisplayName, &dom.Description, &dom.Enabled, &dom.CreateAt, &dom.UpdateAt); err != nil {
			return nil, 0, exception.NewInternalServerError("scan domain record error, %s", err)
		}
		domains = append(domains, &dom)
	}

	total, err := m.domainCount()
	if err != nil {
		return nil, 0, err
	}

	if ps != 0 {
		if total < ps {
			totalP = 1
		} else {
			ok := total % ps
			totalP = total / ps
			if ok != 0 {
				totalP++
			}
		}
	} else {
		totalP = 1
	}

	return domains, totalP, nil
}

func (m *manager) domainCount() (int64, error) {
	count := int64(0)
	if err := m.db.QueryRow("SELECT COUNT(*) FROM domain").Scan(&count); err != nil {
		return 0, exception.NewInternalServerError("count domain record error, %s", err)
	}

	return count, nil
}

// UpdateDomain use to update an domain
func (m *manager) UpdateDomain(id, name, description string) (*domain.Domain, error) {
	return nil, nil
}

// DeleteDomain use to delete an domain from db
func (m *manager) DeleteDomain(id string) error {
	var (
		once sync.Once
		err  error
	)

	once.Do(func() {
		deletePrepare, err = m.db.Prepare("DELETE FROM domain WHERE id = ?")
	})
	if err != nil {
		return exception.NewInternalServerError("prepare delete domain stmt error, %s", err)
	}

	ret, err := deletePrepare.Exec(id)
	if err != nil {
		return exception.NewInternalServerError("delete domain exec sql error, %s", err)
	}
	count, err := ret.RowsAffected()
	if err != nil {
		return exception.NewInternalServerError("get delete row affected error, %s", err)
	}
	if count == 0 {
		return exception.NewBadRequest("domian %s not exist", id)
	}

	return nil
}

func (m *manager) CheckDomainIsExistByID(domainID string) (bool, error) {
	var id string
	if err := m.db.QueryRow("SELECT id FROM domain WHERE id = ?", domainID).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, exception.NewInternalServerError("query single domain error, %s", err)
	}

	return true, nil
}

func (m *manager) CheckDomainIsExistByName(domainName string) (bool, error) {
	var id string
	if err := m.db.QueryRow("SELECT id FROM domain WHERE name = ?", domainName).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, exception.NewInternalServerError("query single domain error, %s", err)
	}

	return true, nil
}
