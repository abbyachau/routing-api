package migration_test

import (
	"path"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/migration"
	"code.cloudfoundry.org/routing-api/migration/fakes"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migration", func() {
	var (
		sqlDB                 *db.SqlDB
		sqlCfg                *config.SqlDB
		mysqlAllocator        testrunner.DbAllocator
		fakeMigration         *fakes.FakeMigration
		fakeLastMigration     *fakes.FakeMigration
		migrations            []migration.Migration
		lastMigrationVersion  int = 10
		firstMigrationVersion int = 1
		logger                lager.Logger
	)

	BeforeEach(func() {
		logger = lager.NewLogger("test-logger")
		mysqlAllocator = testrunner.NewMySQLAllocator()
		mysqlSchema, err := mysqlAllocator.Create()
		Expect(err).NotTo(HaveOccurred())

		sqlCfg = &config.SqlDB{
			Username: "root",
			Password: "password",
			Schema:   mysqlSchema,
			Host:     "localhost",
			Port:     3306,
			Type:     "mysql",
		}

		sqlDB, err = db.NewSqlDB(sqlCfg)
		Expect(err).ToNot(HaveOccurred())

		fakeMigration = new(fakes.FakeMigration)
		fakeLastMigration = new(fakes.FakeMigration)

		fakeMigration.VersionReturns(firstMigrationVersion)
		fakeLastMigration.VersionReturns(lastMigrationVersion)
		migrations = []migration.Migration{}
		migrations = append(migrations, fakeMigration)
		migrations = append(migrations, fakeLastMigration)
	})

	AfterEach(func() {
		mysqlAllocator.Delete()
	})

	Describe("InitializeMigrations", func() {
		var etcdConfig *config.Etcd
		BeforeEach(func() {
			basePath, err := filepath.Abs(path.Join("..", "fixtures", "etcd-certs"))
			Expect(err).NotTo(HaveOccurred())

			serverSSLConfig := &etcdstorerunner.SSLConfig{
				CertFile: filepath.Join(basePath, "server.crt"),
				KeyFile:  filepath.Join(basePath, "server.key"),
				CAFile:   filepath.Join(basePath, "etcd-ca.crt"),
			}

			etcdPort := 4001 + GinkgoParallelNode()
			etcdRunner := etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, serverSSLConfig)

			etcdConfig = &config.Etcd{
				RequireSSL: true,
				CertFile:   filepath.Join(basePath, "client.crt"),
				KeyFile:    filepath.Join(basePath, "client.key"),
				CAFile:     filepath.Join(basePath, "etcd-ca.crt"),
				NodeURLS:   etcdRunner.NodeURLS(),
			}
		})

		It("initializes all possible migrations", func() {
			done := make(chan struct{})
			defer close(done)
			migrations := migration.InitializeMigrations(etcdConfig, done, logger)
			Expect(migrations).To(HaveLen(2))

			Expect(migrations[0]).To(BeAssignableToTypeOf(&migration.V0InitMigration{}))
			Expect(migrations[1]).To(BeAssignableToTypeOf(&migration.V1EtcdMigration{}))
		})
	})

	Describe("RunMigrations", func() {
		Context("when no SqlDB exists", func() {
			It("should be a no-op", func() {
				err := migration.RunMigrations(nil, migrations)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeMigration.RunCallCount()).To(Equal(0))
				Expect(fakeLastMigration.RunCallCount()).To(Equal(0))
			})
		})
		Context("when no migration table exists", func() {
			It("should create the migration table and set the target version to last migration version", func() {
				err := migration.RunMigrations(sqlDB, migrations)
				Expect(err).ToNot(HaveOccurred())
				gormClient := sqlDB.Client.(*gorm.DB)
				Expect(gormClient.HasTable(&migration.MigrationData{})).To(BeTrue())

				var migrationVersions []migration.MigrationData
				gormClient.Find(&migrationVersions)

				Expect(migrationVersions).To(HaveLen(1))

				migrationVersion := migrationVersions[0]
				Expect(migrationVersion.MigrationKey).To(Equal(migration.MigrationKey))
				Expect(migrationVersion.CurrentVersion).To(Equal(lastMigrationVersion))
				Expect(migrationVersion.TargetVersion).To(Equal(lastMigrationVersion))
			})
			It("should run all the migrations up to the current version", func() {
				err := migration.RunMigrations(sqlDB, migrations)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeMigration.RunCallCount()).To(Equal(1))
				Expect(fakeLastMigration.RunCallCount()).To(Equal(1))
			})
		})

		Context("when a migration table exists", func() {
			BeforeEach(func() {
				gormClient := sqlDB.Client.(*gorm.DB)
				gormClient.AutoMigrate(&migration.MigrationData{})
			})

			Context("when a migration is necessary", func() {
				Context("when another routing-api has already started the migration", func() {
					BeforeEach(func() {
						migrationData := migration.MigrationData{
							MigrationKey:   migration.MigrationKey,
							CurrentVersion: -1,
							TargetVersion:  lastMigrationVersion,
						}

						err := sqlDB.Client.Create(migrationData).Error
						Expect(err).ToNot(HaveOccurred())
					})

					It("should not update the migration data", func() {
						err := migration.RunMigrations(sqlDB, migrations)
						Expect(err).ToNot(HaveOccurred())

						var migrationVersions []migration.MigrationData
						sqlDB.Client.Find(&migrationVersions)

						Expect(migrationVersions).To(HaveLen(1))

						migrationVersion := migrationVersions[0]
						Expect(migrationVersion.MigrationKey).To(Equal(migration.MigrationKey))
						Expect(migrationVersion.CurrentVersion).To(Equal(-1))
						Expect(migrationVersion.TargetVersion).To(Equal(lastMigrationVersion))
					})

					It("should not run any migrations", func() {
						err := migration.RunMigrations(sqlDB, migrations)
						Expect(err).ToNot(HaveOccurred())

						Expect(fakeMigration.RunCallCount()).To(BeZero())
					})
				})

				Context("when the migration has not been started", func() {
					BeforeEach(func() {
						migrationData := migration.MigrationData{
							MigrationKey:   migration.MigrationKey,
							CurrentVersion: 1,
							TargetVersion:  1,
						}

						err := sqlDB.Client.Create(migrationData).Error
						Expect(err).ToNot(HaveOccurred())
					})

					It("should update the migration data with the target version", func() {
						err := migration.RunMigrations(sqlDB, migrations)
						Expect(err).ToNot(HaveOccurred())

						var migrationVersions []migration.MigrationData
						sqlDB.Client.Find(&migrationVersions)

						Expect(migrationVersions).To(HaveLen(1))

						migrationVersion := migrationVersions[0]
						Expect(migrationVersion.MigrationKey).To(Equal(migration.MigrationKey))
						Expect(migrationVersion.CurrentVersion).To(Equal(lastMigrationVersion))
						Expect(migrationVersion.TargetVersion).To(Equal(lastMigrationVersion))
					})

					It("should run all the migrations up to the current version", func() {
						err := migration.RunMigrations(sqlDB, migrations)
						Expect(err).ToNot(HaveOccurred())
						Expect(fakeMigration.RunCallCount()).To(Equal(0))
						Expect(fakeLastMigration.RunCallCount()).To(Equal(1))
					})
				})
			})

			Context("when a migration is unnecessary", func() {
				BeforeEach(func() {
					migrationData := migration.MigrationData{
						MigrationKey:   migration.MigrationKey,
						CurrentVersion: lastMigrationVersion,
						TargetVersion:  lastMigrationVersion,
					}

					err := sqlDB.Client.Create(migrationData).Error
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not update the migration data", func() {
					err := migration.RunMigrations(sqlDB, migrations)
					Expect(err).ToNot(HaveOccurred())

					var migrationVersions []migration.MigrationData
					sqlDB.Client.Find(&migrationVersions)

					Expect(migrationVersions).To(HaveLen(1))

					migrationVersion := migrationVersions[0]
					Expect(migrationVersion.MigrationKey).To(Equal(migration.MigrationKey))
					Expect(migrationVersion.CurrentVersion).To(Equal(lastMigrationVersion))
					Expect(migrationVersion.TargetVersion).To(Equal(lastMigrationVersion))
				})

				It("should not run any migrations", func() {
					err := migration.RunMigrations(sqlDB, migrations)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeMigration.RunCallCount()).To(BeZero())
					Expect(fakeLastMigration.RunCallCount()).To(BeZero())
				})
			})
		})
	})
})
