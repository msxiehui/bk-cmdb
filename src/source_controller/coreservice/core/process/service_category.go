/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.,
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the ",License",); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an ",AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package process

import (
	"strconv"

	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/mapstr"
	"configcenter/src/common/metadata"
	"configcenter/src/source_controller/coreservice/core"
)

func (p *processOperation) CreateServiceCategory(ctx core.ContextParams, category metadata.ServiceCategory) (*metadata.ServiceCategory, error) {
	// base attribute validate
	if field, err := category.Validate(); err != nil {
		blog.Errorf("CreateServiceCategory failed, validation failed, code: %d, err: %+v, rid: %s", common.CCErrCommParamsInvalid, err, ctx.ReqID)
		err := ctx.Error.New(common.CCErrCommParamsInvalid, field)
		return nil, err
	}
	
	var bizID int64
	var err error
	if bizID, err = p.validateBizID(ctx, category.Metadata); err != nil {
		blog.Errorf("CreateServiceCategory failed, validation failed, code: %d, err: %+v, rid: %s", common.CCErrCommParamsInvalid, err, ctx.ReqID)
		return nil, ctx.Error.New(common.CCErrCommParamsInvalid, "metadata.label.bk_biz_id")
	}

	// keep metadata clean
	category.Metadata = metadata.NewMetaDataFromBusinessID(strconv.FormatInt(bizID, 10))

	category.RootID = 0
	if category.ParentID > 0 {
		parentCategory, err := p.GetServiceCategory(ctx, category.ParentID)
		if err != nil {
			blog.Errorf("CreateServiceCategory failed, parent id invalid, code: %d, err: %+v, rid: %s", common.CCErrCommParamsInvalid, err, ctx.ReqID)
			return nil, ctx.Error.New(common.CCErrCommParamsInvalid, "metadata.label.bk_biz_id")
		}
		category.RootID = parentCategory.RootID
	}

	// generate id field
	id, err := p.dbProxy.NextSequence(ctx, common.BKTableNameServiceCategory)
	if nil != err {
		blog.Errorf("CreateServiceCategory failed, generate id failed, err: %+v, rid: %s", err, ctx.ReqID)
		return nil, err
	}
	category.ID = int64(id)
	
	// set root id
	if category.RootID == 0 {
		category.RootID = category.ID
	}
	
	category.IsBuiltIn = false
	category.SupplierAccount = ctx.SupplierAccount
	
	if err := p.dbProxy.Table(common.BKTableNameServiceCategory).Insert(ctx.Context, &category); nil != err {
		blog.Errorf("mongodb failed, table: %s, err: %+v, rid: %s", common.BKTableNameServiceCategory, err, ctx.ReqID)
		return  nil, err
	}
	
	return &category, nil
}

func (p *processOperation) GetServiceCategory(ctx core.ContextParams, categoryID int64) (*metadata.ServiceCategory, error) {
	category := metadata.ServiceCategory{}
	
	filter := map[string]int64{common.BKFieldID: categoryID}
	if err := p.dbProxy.Table(common.BKTableNameServiceCategory).Find(filter).One(ctx.Context, &category); nil != err {
		blog.Errorf("GetServiceCategory failed, mongodb failed, table: %s, err: %+v, rid: %s", common.BKTableNameServiceCategory, err, ctx.ReqID)
		if err.Error() == "document not found" {
			return nil, ctx.Error.CCError(common.CCErrCommNotFound)
		}
		return  nil, err
	}
	
	return &category, nil
}

func (p *processOperation) UpdateServiceCategory(ctx core.ContextParams, categoryID int64, input metadata.ServiceCategory) (*metadata.ServiceCategory, error) {
	category, err := p.GetServiceCategory(ctx, categoryID)
	if err != nil {
		return nil, err
	}

	// update fields to local object
	category.Name = input.Name
	if field, err := category.Validate(); err != nil {
		blog.Errorf("UpdateServiceCategory failed, validation failed, code: %d, err: %+v, rid: %s", common.CCErrCommParamsInvalid, err, ctx.ReqID)
		err := ctx.Error.New(common.CCErrCommParamsInvalid, field)
		return nil, err
	}
	
	if category.IsBuiltIn == true {
		blog.Errorf("UpdateServiceCategory failed, forbidden update built-in category, code: %d, rid: %s", common.CCErrCommOperateBuiltInItemForbidden, ctx.ReqID)
		err := ctx.Error.CCError(common.CCErrCommOperateBuiltInItemForbidden)
		return nil, err
	}
	
	// do update
	filter := map[string]int64{common.BKFieldID: categoryID}
	if err := p.dbProxy.Table(common.BKTableNameServiceCategory).Update(ctx, filter, category); nil != err {
		blog.Errorf("UpdateServiceCategory failed, mongodb failed, table: %s, err: %+v, rid: %s", common.BKTableNameServiceCategory, err, ctx.ReqID)
		return  nil, err
	}
	return category, nil
}

func (p *processOperation) ListServiceCategories(ctx core.ContextParams, bizID int64, withStatistics bool) (*metadata.MultipleServiceCategory, error) {
	md := metadata.NewMetaDataFromBusinessID(strconv.FormatInt(bizID, 10))
	filter := map[string]mapstr.MapStr{
		"metadata": md.ToMapStr(),
	}
	
	
	categories := make([]metadata.ServiceCategory, 0)
	if err := p.dbProxy.Table(common.BKTableNameServiceCategory).Find(filter).All(ctx.Context, &categories); nil != err {
		blog.Errorf("ListServiceCategories failed, mongodb failed, table: %s, err: %+v, rid: %s", common.BKTableNameServiceCategory, err, ctx.ReqID)
		return  nil, err
	}
	
	result := &metadata.MultipleServiceCategory{
		Count: int64(len(categories)),
		Info:  categories,
	}
	return result, nil
}

func (p *processOperation) DeleteServiceCategory(ctx core.ContextParams, categoryID int64) error {
	category, err := p.GetServiceCategory(ctx, categoryID)
	if err != nil {
		blog.Errorf("DeleteServiceCategory failed, GetServiceCategory failed, categoryID: %d, err: %+v, rid: %s", categoryID, err, ctx.ReqID)
		return err
	}

	if category.IsBuiltIn == true {
		blog.Errorf("DeleteServiceCategory failed, forbidden delete built-in category, code: %d, rid: %s", common.CCErrCommOperateBuiltInItemForbidden, ctx.ReqID)
		err := ctx.Error.CCError(common.CCErrCommOperateBuiltInItemForbidden)
		return err
	}

	// category that has sub category shouldn't be removed
	childrenFilter := map[string]int64{"parent_id": category.ID}
	childrenCount, err := p.dbProxy.Table(common.BKTableNameServiceCategory).Find(childrenFilter).Count(ctx.Context)
	if nil != err {
		blog.Errorf("DeleteServiceCategory failed, mongodb failed, table: %s, err: %+v, rid: %s", common.BKTableNameServiceCategory, err, ctx.ReqID)
		return  err
	}
	if childrenCount > 0 {
		blog.Errorf("DeleteServiceCategory failed, forbidden delete category has children node, childrenCount: %d, code: %d, rid: %s", childrenCount, common.CCErrCommOperateBuiltInItemForbidden, ctx.ReqID)
		err := ctx.Error.CCError(common.CCErrCommRemoveRecordHasChildrenForbidden)
		return err
	}

	// category that referenced by service template shouldn't be removed
	usageFilter := map[string]int64{"service_category_id": category.ID}
	usageCount, err := p.dbProxy.Table(common.BKTableNameServiceTemplate).Find(usageFilter).Count(ctx.Context)
	if nil != err {
		blog.Errorf("DeleteServiceCategory failed, mongodb failed, table: %s, err: %+v, rid: %s", common.BKTableNameServiceTemplate, err, ctx.ReqID)
		return  err
	}
	if usageCount > 0 {
		blog.Errorf("DeleteServiceCategory failed, forbidden delete category be referenced, code: %d, rid: %s", common.CCErrCommRemoveRecordHasChildrenForbidden, ctx.ReqID)
		err := ctx.Error.CCError(common.CCErrCommRemoveRecordHasChildrenForbidden)
		return err
	}

	deleteFilter := map[string]int64{common.BKFieldID: category.ID}
	if err := p.dbProxy.Table(common.BKTableNameServiceCategory).Delete(ctx, deleteFilter); nil != err {
		blog.Errorf("DeleteServiceCategory failed, mongodb failed, table: %s, err: %+v, rid: %s", common.BKTableNameServiceCategory, err, ctx.ReqID)
		return  err
	}
	return nil
}