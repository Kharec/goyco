package repositories

import "gorm.io/gorm"

func ApplyPagination(query *gorm.DB, limit, offset int) *gorm.DB {
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	return query
}
