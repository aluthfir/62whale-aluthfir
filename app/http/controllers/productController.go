package controllers

import (
	"errors"
	"net/http"
	"whale/62teknologi-golang-utility/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/gosimple/slug"
	"gorm.io/gorm"
)

var label string
var table string

func init() {
	label = "product"
	table = "products"
}

func FindProduct(ctx *gin.Context) {
	var value map[string]interface{}
	err := utils.DB.Table(table).Where("id = ?", ctx.Param("id")).Take(&value).Error

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ResponseData("error", err.Error(), nil))
		return
	}

	if value["id"] == nil {
		ctx.JSON(http.StatusBadRequest, utils.ResponseData("error", label+" not found", nil))
		return
	}

	transformer, _ := utils.JsonFileParser("transformers/response/" + label + "/find.json")
	customResponse := transformer["product"]

	utils.MapValuesShifter(transformer, value)

	if customResponse != nil {
		utils.MapValuesShifter(customResponse.(map[string]any), value)
	}

	ctx.JSON(http.StatusOK, utils.ResponseData("success", "find "+label+" success", transformer))
}

func FindProducts(ctx *gin.Context) {
	var values []map[string]interface{}
	err := utils.DB.Table(table).Find(&values).Error

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ResponseData("error", err.Error(), nil))
		return
	}

	var customResponses []map[string]any
	for _, value := range values {
		transformer, _ := utils.JsonFileParser("transformers/response/product/find.json")
		customResponse := transformer["product"]

		utils.MapValuesShifter(transformer, value)
		if customResponse != nil {
			utils.MapValuesShifter(customResponse.(map[string]any), value)
		}
		customResponses = append(customResponses, transformer)
	}

	ctx.JSON(http.StatusOK, utils.ResponseData("success", "find "+label+"s success", customResponses))
}

func UpdateProduct(ctx *gin.Context) {
	transformer, _ := utils.JsonFileParser("transformers/request/" + label + "/update.json")
	var input map[string]any

	if err := ctx.BindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ResponseData("error", err.Error(), nil))
		return
	}

	if validation, err := utils.Validate(input, transformer); err {
		ctx.JSON(http.StatusOK, utils.ResponseData("failed", "validation", validation.Errors))
		return
	}

	utils.MapValuesShifter(transformer, input)
	utils.MapNullValuesRemover(transformer)

	sku, sku_exist := transformer["skus"]
	group, groups_exist := transformer["groups"]

	delete(transformer, "skus")
	delete(transformer, "groups")

	name, _ := transformer["name"].(string)
	transformer["slug"] = slug.Make(name)

	queryResult := utils.DB.Table(table).Where("id = ?", ctx.Param("id")).Updates(&transformer)

	if sku_exist {
		skus := utils.Prepare1toM("product_id", transformer["id"], sku)
		utils.DB.Table("product_skus").Create(&skus)
	}

	if groups_exist {
		var deleteResult = map[string]any{
			"product_id": ctx.Param("id"),
		}
		utils.DB.Table("products_groups").Where("product_id = ?", ctx.Param("id")).Delete(&deleteResult)
		groups := utils.PrepareMtoM("product_id", ctx.Param("id"), "product_group_id", group)
		utils.DB.Table("products_groups").Create(&groups)
	}

	var mysqlErr *mysql.MySQLError

	if queryResult.Error != nil || errors.As(queryResult.Error, &mysqlErr) && mysqlErr.Number == 1062 {
		ctx.JSON(http.StatusBadRequest, utils.ResponseData("error", queryResult.Error.Error(), nil))
		return
	}

	// todo : make a better response!
	FindProduct(ctx)
}

func CreateProduct(ctx *gin.Context) {
	transformer, _ := utils.JsonFileParser("transformers/request/" + label + "/create.json")
	var input map[string]any

	if err := ctx.BindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ResponseData("error", err.Error(), nil))
		return
	}

	if validation, err := utils.Validate(input, transformer); err {
		ctx.JSON(http.StatusOK, utils.ResponseData("failed", "validation", validation.Errors))
		return
	}

	utils.MapValuesShifter(transformer, input)
	utils.MapNullValuesRemover(transformer)

	sku, sku_exist := input["skus"]
	group, groups_exist := input["groups"]

	delete(transformer, "skus")
	delete(transformer, "groups")

	name, _ := transformer["name"].(string)
	transformer["slug"] = slug.Make(name)

	if err := utils.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(table).Create(&transformer).Error; err != nil {
			return err
		}

		if sku_exist || groups_exist {
			tx.Table(table).Where("slug = ?", transformer["slug"]).Take(&transformer)

			if sku_exist {
				skus := utils.Prepare1toM("product_id", transformer["id"], sku)

				if err := tx.Table("product_skus").Create(&skus).Error; err != nil {
					return err
				}
			}

			if groups_exist {
				groups := utils.PrepareMtoM("product_id", transformer["id"], "product_group_id", group)

				if err := tx.Table("products_groups").Create(&groups).Error; err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ResponseData("error", err.Error(), nil))
		return
	}

	ctx.JSON(http.StatusOK, utils.ResponseData("success", "create "+label+" success", transformer))
}
