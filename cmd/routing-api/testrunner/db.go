package testrunner

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

var etcdVersion = "etcdserver\":\"2.1.1"

type DbAllocator interface {
	Create() (string, error)
	Reset() error
	Delete() error
}

type mysqlAllocator struct {
	sqlDB      *sql.DB
	schemaName string
}

type postgresAllocator struct {
	sqlDB      *sql.DB
	schemaName string
}

func NewPostgresAllocator() DbAllocator {
	sqlDBName := fmt.Sprintf("test%d", rand.Int())
	return &postgresAllocator{schemaName: sqlDBName}
}
func NewMySQLAllocator() DbAllocator {
	sqlDBName := fmt.Sprintf("test%d", rand.Int())
	return &mysqlAllocator{schemaName: sqlDBName}
}

type etcdAllocator struct {
	port        int
	etcdAdapter storeadapter.StoreAdapter
	etcdRunner  *etcdstorerunner.ETCDClusterRunner
}

func NewEtcdAllocator(port int) DbAllocator {
	return &etcdAllocator{port: port}
}

func (a *postgresAllocator) Create() (string, error) {
	var err error
	a.sqlDB, err = sql.Open("postgres", "postgres://postgres:@localhost/?sslmode=disable")
	if err != nil {
		return "", err
	}
	err = a.sqlDB.Ping()
	if err != nil {
		return "", err
	}

	_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
	if err != nil {
		return "", err
	}

	return a.schemaName, nil
}
func (a *postgresAllocator) Reset() error {
	_, err := a.sqlDB.Exec(fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
	WHERE datname = '%s'`, a.schemaName))
	_, err = a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	if err != nil {
		return err
	}

	_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
	return err
}

func (a *postgresAllocator) Delete() error {
	defer a.sqlDB.Close()
	_, err := a.sqlDB.Exec(fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
	WHERE datname = '%s'`, a.schemaName))
	if err != nil {
		return err
	}
	_, err = a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	return err
}

func (a *mysqlAllocator) Create() (string, error) {
	var err error
	a.sqlDB, err = sql.Open("mysql", "root:password@/")
	if err != nil {
		return "", err
	}
	err = a.sqlDB.Ping()
	if err != nil {
		return "", err
	}

	_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
	if err != nil {
		return "", err
	}

	return a.schemaName, nil
}

func (a *mysqlAllocator) Reset() error {
	_, err := a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	if err != nil {
		return err
	}

	_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
	return err
}

func (a *mysqlAllocator) Delete() error {
	defer a.sqlDB.Close()
	_, err := a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	return err
}

func (e *etcdAllocator) Create() (string, error) {
	e.etcdRunner = etcdstorerunner.NewETCDClusterRunner(e.port, 1, nil)
	e.etcdRunner.Start()

	etcdVersionUrl := e.etcdRunner.NodeURLS()[0] + "/version"
	resp, err := http.Get(etcdVersionUrl)
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// response body: {"etcdserver":"2.1.1","etcdcluster":"2.1.0"}
	if !strings.Contains(string(body), etcdVersion) {
		return "", errors.New("Incorrect etcd version")
	}

	e.etcdAdapter = e.etcdRunner.Adapter(nil)

	etcdUrl := fmt.Sprintf("http://127.0.0.1:%d", e.port)
	return etcdUrl, nil
}

func (e *etcdAllocator) Reset() error {
	e.etcdRunner.Reset()
	return nil
}

func (e *etcdAllocator) Delete() error {
	e.etcdAdapter.Disconnect()
	e.etcdRunner.Reset()
	e.etcdRunner.Stop()
	e.etcdRunner.KillWithFire()
	return nil
}
