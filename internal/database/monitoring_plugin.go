package database

import (
	"context"
	"time"

	"gorm.io/gorm"
	"goyco/internal/middleware"
)

type contextKey string

const gormOperationStartKey contextKey = "gorm_operation_start"

type GormDBMonitor struct {
	monitor middleware.DBMonitor
}

func NewGormDBMonitor(monitor middleware.DBMonitor) *GormDBMonitor {
	return &GormDBMonitor{
		monitor: monitor,
	}
}

func (g *GormDBMonitor) Name() string {
	return "db_monitor"
}

func (g *GormDBMonitor) Initialize(db *gorm.DB) error {
	if err := db.Callback().Create().Before("gorm:create").Register("db_monitor:before_create", g.beforeCreate); err != nil {
		return err
	}
	if err := db.Callback().Create().After("gorm:create").Register("db_monitor:after_create", g.afterCreate); err != nil {
		return err
	}

	if err := db.Callback().Query().Before("gorm:query").Register("db_monitor:before_query", g.beforeQuery); err != nil {
		return err
	}
	if err := db.Callback().Query().After("gorm:query").Register("db_monitor:after_query", g.afterQuery); err != nil {
		return err
	}

	if err := db.Callback().Update().Before("gorm:update").Register("db_monitor:before_update", g.beforeUpdate); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register("db_monitor:after_update", g.afterUpdate); err != nil {
		return err
	}

	if err := db.Callback().Delete().Before("gorm:delete").Register("db_monitor:before_delete", g.beforeDelete); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("db_monitor:after_delete", g.afterDelete); err != nil {
		return err
	}

	if err := db.Callback().Row().Before("gorm:row").Register("db_monitor:before_row", g.beforeRow); err != nil {
		return err
	}
	if err := db.Callback().Row().After("gorm:row").Register("db_monitor:after_row", g.afterRow); err != nil {
		return err
	}

	if err := db.Callback().Raw().Before("gorm:raw").Register("db_monitor:before_raw", g.beforeRaw); err != nil {
		return err
	}
	if err := db.Callback().Raw().After("gorm:raw").Register("db_monitor:after_raw", g.afterRaw); err != nil {
		return err
	}

	return nil
}

func (g *GormDBMonitor) beforeCreate(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	ctx := context.WithValue(db.Statement.Context, gormOperationStartKey, time.Now())
	db.Statement.Context = ctx
}

func (g *GormDBMonitor) afterCreate(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	g.logOperation(db, "CREATE")
}

func (g *GormDBMonitor) beforeQuery(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	ctx := context.WithValue(db.Statement.Context, gormOperationStartKey, time.Now())
	db.Statement.Context = ctx
}

func (g *GormDBMonitor) afterQuery(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	g.logOperation(db, "SELECT")
}

func (g *GormDBMonitor) beforeUpdate(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	ctx := context.WithValue(db.Statement.Context, gormOperationStartKey, time.Now())
	db.Statement.Context = ctx
}

func (g *GormDBMonitor) afterUpdate(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	g.logOperation(db, "UPDATE")
}

func (g *GormDBMonitor) beforeDelete(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	ctx := context.WithValue(db.Statement.Context, gormOperationStartKey, time.Now())
	db.Statement.Context = ctx
}

func (g *GormDBMonitor) afterDelete(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	g.logOperation(db, "DELETE")
}

func (g *GormDBMonitor) beforeRow(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	ctx := context.WithValue(db.Statement.Context, gormOperationStartKey, time.Now())
	db.Statement.Context = ctx
}

func (g *GormDBMonitor) afterRow(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	g.logOperation(db, "ROW")
}

func (g *GormDBMonitor) beforeRaw(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	ctx := context.WithValue(db.Statement.Context, gormOperationStartKey, time.Now())
	db.Statement.Context = ctx
}

func (g *GormDBMonitor) afterRaw(db *gorm.DB) {
	if g.monitor == nil {
		return
	}

	g.logOperation(db, "RAW")
}

func (g *GormDBMonitor) logOperation(db *gorm.DB, operation string) {
	if g.monitor == nil {
		return
	}

	startTime, ok := db.Statement.Context.Value(gormOperationStartKey).(time.Time)
	if !ok {
		return
	}

	duration := time.Since(startTime)

	query := g.buildQueryString(db, operation)

	g.monitor.LogQuery(query, duration, db.Error)
}

func (g *GormDBMonitor) buildQueryString(db *gorm.DB, operation string) string {
	if db.Statement.SQL.String() != "" {
		return db.Statement.SQL.String()
	}

	query := operation

	if db.Statement.Table != "" {
		query += " FROM " + db.Statement.Table
	}

	if db.Statement.Model != nil {

		if stmt := db.Statement; stmt.Schema != nil {
			query = operation + " " + stmt.Schema.Table
		}
	}

	return query
}
