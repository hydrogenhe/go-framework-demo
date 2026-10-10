package es

import (
	"github.com/olivere/elastic/v7"
	"strconv"
)

type UserSearchOptions struct {
	ID    int64
	Name  string
	Email string
}

func BuildUserElasticQuery(opt UserSearchOptions) elastic.Query {
	q := elastic.NewBoolQuery()
	if opt.ID > 0 {
		q.Filter(elastic.NewTermQuery("user_id", strconv.FormatInt(opt.ID, 10)))
	}
	if opt.Name != "" {
		q.Must(elastic.NewMatchQuery("name", opt.Name))
	}
	if opt.Email != "" {
		q.Filter(elastic.NewTermQuery("email.keyword", opt.Email))
	}
	return q
}

type OrderSearchOptions struct {
	ID         int64
	OrderNo    string
	UserID     int64
	ProductIDs []int64
}

func BuildOrderElasticQuery(opt OrderSearchOptions) elastic.Query {
	q := elastic.NewBoolQuery()
	if opt.ID > 0 {
		q.Filter(elastic.NewTermQuery("id", strconv.FormatInt(opt.ID, 10)))
	}
	if opt.OrderNo != "" {
		q.Filter(elastic.NewTermQuery("order_no", opt.OrderNo))
	}
	if opt.UserID > 0 {
		q.Filter(elastic.NewTermQuery("user_id", strconv.FormatInt(opt.UserID, 10)))
	}
	if len(opt.ProductIDs) > 0 {
		ids := []interface{}{}
		for _, id := range opt.ProductIDs {
			if id > 0 {
				ids = append(ids, strconv.FormatInt(id, 10))
			}
		}
		if len(ids) > 0 {
			q.Filter(elastic.NewNestedQuery("items", elastic.NewTermsQuery("items.product_id", ids...)))
		}
	}
	return q
}
