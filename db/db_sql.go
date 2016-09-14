package db

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"

	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/models"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

type SqlDB struct {
	Client Client
}

const DeleteError = "Delete Fails: TCP Route Mapping does not exist"

var _ DB = &SqlDB{}

func NewSqlDB(cfg *config.SqlDB) (DB, error) {
	if cfg == nil {
		return nil, errors.New("SQL configuration cannot be nil")
	}
	connectionString := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Schema)

	db, err := gorm.Open(cfg.Type, connectionString)
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&models.RouterGroupDB{}, &models.TcpRouteMapping{})
	return &SqlDB{Client: db}, nil
}

func (s *SqlDB) ReadRouterGroups() (models.RouterGroups, error) {
	routerGroupsDB := models.RouterGroupsDB{}
	routerGroups := models.RouterGroups{}
	err := s.Client.Find(&routerGroupsDB).Error
	if err == nil {
		routerGroups = routerGroupsDB.ToRouterGroups()
	}

	return routerGroups, err
}

func (s *SqlDB) ReadRouterGroup(guid string) (models.RouterGroup, error) {
	routerGroupDB := models.RouterGroupDB{}
	routerGroup := models.RouterGroup{}
	err := s.Client.Where("guid = ?", guid).First(&routerGroupDB).Error
	if err == nil {
		routerGroup = routerGroupDB.ToRouterGroup()
	}

	if recordNotFound(err) {
		err = nil
	}
	return routerGroup, err
}

func (s *SqlDB) SaveRouterGroup(routerGroup models.RouterGroup) error {
	existingRouterGroup, err := s.ReadRouterGroup(routerGroup.Guid)
	if err != nil {
		return err
	}

	routerGroupDB := models.NewRouterGroupDB(routerGroup)
	if existingRouterGroup.Guid == routerGroup.Guid {
		updateRouterGroup(&existingRouterGroup, &routerGroup)
		routerGroupDB = models.NewRouterGroupDB(existingRouterGroup)
		err = s.Client.Save(&routerGroupDB).Error
	} else {
		err = s.Client.Create(&routerGroupDB).Error
	}

	return err
}

func updateRouterGroup(existingRouterGroup, currentRouterGroup *models.RouterGroup) {
	if currentRouterGroup.Type != "" {
		existingRouterGroup.Type = currentRouterGroup.Type
	}
	if currentRouterGroup.Name != "" {
		existingRouterGroup.Name = currentRouterGroup.Name
	}
	if currentRouterGroup.ReservablePorts != "" {
		existingRouterGroup.ReservablePorts = currentRouterGroup.ReservablePorts
	}
}

func updateTcpRouteMapping(existingTcpRouteMapping *models.TcpRouteMapping, currentTcpRouteMapping *models.TcpRouteMapping) {
	if currentTcpRouteMapping.ModificationTag != (models.ModificationTag{}) {
		existingTcpRouteMapping.ModificationTag = currentTcpRouteMapping.ModificationTag
	}
	if currentTcpRouteMapping.TTL != nil {
		existingTcpRouteMapping.TTL = currentTcpRouteMapping.TTL
	}
}

func notImplementedError() error {
	pc, _, _, _ := runtime.Caller(1)
	fnName := runtime.FuncForPC(pc).Name()
	return errors.New(fmt.Sprintf("function not implemented: %s", fnName))
}

func (s *SqlDB) ReadRoutes() ([]models.Route, error) {
	return nil, notImplementedError()
}
func (s *SqlDB) SaveRoute(route models.Route) error {
	return notImplementedError()
}
func (s *SqlDB) DeleteRoute(route models.Route) error {
	return notImplementedError()
}

func (s *SqlDB) ReadTcpRouteMappings() ([]models.TcpRouteMapping, error) {
	var tcpRoutes []models.TcpRouteMapping
	err := s.Client.Find(&tcpRoutes).Error
	if err != nil {
		return nil, err
	}
	return tcpRoutes, nil
}

func (s *SqlDB) ReadTcpRouteMapping(tcpMapping models.TcpRouteMapping) (models.TcpRouteMapping, error) {
	var routes []models.TcpRouteMapping
	var tcpRoute models.TcpRouteMapping
	err := s.Client.Where("host_ip = ? and host_port = ? and external_port = ?",
		tcpMapping.HostIP, tcpMapping.HostPort, tcpMapping.TcpRoute.ExternalPort).Find(&routes).Error

	if err != nil {
		return tcpRoute, err
	}
	count := len(routes)
	if count > 1 || count < 0 {
		return tcpRoute, errors.New("Have duplicate tcp route mappings")
	}
	if count == 1 {
		tcpRoute = routes[0]
	}

	return tcpRoute, err
}

func (s *SqlDB) SaveTcpRouteMapping(tcpMapping models.TcpRouteMapping) error {
	var existingTcpRouteMapping models.TcpRouteMapping
	existingTcpRouteMapping, err := s.ReadTcpRouteMapping(tcpMapping)
	if err != nil {
		return err
	}
	if existingTcpRouteMapping != (models.TcpRouteMapping{}) {
		updateTcpRouteMapping(&existingTcpRouteMapping, &tcpMapping)
		return s.Client.Save(&existingTcpRouteMapping).Error
	}
	return s.Client.Create(&tcpMapping).Error
}

func (s *SqlDB) DeleteTcpRouteMapping(tcpMapping models.TcpRouteMapping) error {
	tcpRoute, err := s.ReadTcpRouteMapping(tcpMapping)
	if err != nil {
		return err
	}
	if tcpRoute == (models.TcpRouteMapping{}) {
		return errors.New(DeleteError)
	}
	return s.Client.Delete(&tcpMapping).Error
}
func (s *SqlDB) Connect() error {
	return notImplementedError()
}

func (s *SqlDB) CancelWatches() {}
func (s *SqlDB) WatchRouteChanges(filter string) (<-chan Event, <-chan error, context.CancelFunc) {
	return nil, nil, nil
}

func recordNotFound(err error) bool {
	if err == gorm.ErrRecordNotFound {
		return true
	}
	return false
}
