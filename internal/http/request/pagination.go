package request

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// ParsePagination 从查询参数解析分页，并统一应用默认值与上限。
func ParsePagination(c fiber.Ctx) (page, pageSize int) {
	page, _ = strconv.Atoi(c.Query("page", "1"))
	pageSize, _ = strconv.Atoi(c.Query("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
