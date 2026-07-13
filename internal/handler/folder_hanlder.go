package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"

	"github.com/go-sql-driver/mysql"
)

// FolderHandler ใช้จัดการ API ที่เกี่ยวข้องกับโฟลเดอร์
type FolderHandler struct {
	folderRepo       *repository.FolderRepository
	fileRepo         *repository.FileRepository
	sharedFolderRepo *repository.SharedFolderRepository
}

// NewFolderHandler สร้าง FolderHandler
func NewFolderHandler(
	folderRepo *repository.FolderRepository,
	fileRepo *repository.FileRepository,
	sharedFolderRepo *repository.SharedFolderRepository,
) *FolderHandler {
	return &FolderHandler{
		folderRepo:       folderRepo,
		fileRepo:         fileRepo,
		sharedFolderRepo: sharedFolderRepo,
	}
}

// Create สร้างโฟลเดอร์ใหม่
// POST /api/folders
func (h *FolderHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	// ดึง user จาก JWT claims
	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	// จำกัดขนาด JSON body
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		64*1024,
	)

	var req models.CreateFolderRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
	}

	// ตรวจสอบชื่อโฟลเดอร์
	if err := validateFolderName(req.FolderName); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	// ถ้ามี parent_id ต้องตรวจว่าโฟลเดอร์แม่มีอยู่จริง
	// และเป็นของผู้ใช้ที่กำลัง Login
	if req.ParentID != nil {
		if *req.ParentID <= 0 {
			response.Error(
				w,
				http.StatusBadRequest,
				"รหัสโฟลเดอร์แม่ไม่ถูกต้อง",
			)

			return
		}

		_, err := h.folderRepo.FindByIDAndUserID(
			r.Context(),
			*req.ParentID,
			claims.UserID,
		)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.Error(
					w,
					http.StatusNotFound,
					"ไม่พบโฟลเดอร์แม่",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"ไม่สามารถตรวจสอบโฟลเดอร์แม่ได้",
			)
			return
		}
	}

	folder := &models.UserFolder{
		UserID:     claims.UserID,
		ParentID:   req.ParentID,
		FolderName: req.FolderName,
	}

	if err := h.folderRepo.Create(
		r.Context(),
		folder,
	); err != nil {
		// Error 1062 คือข้อมูลชนกับ UNIQUE KEY
		// หมายถึงมีชื่อโฟลเดอร์นี้ในตำแหน่งเดียวกันแล้ว
		if isDuplicateEntry(err) {
			response.Error(
				w,
				http.StatusConflict,
				"มีโฟลเดอร์ชื่อนี้อยู่แล้ว",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถสร้างโฟลเดอร์ได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusCreated,
		map[string]interface{}{
			"message": "สร้างโฟลเดอร์สำเร็จ",
			"folder":  models.NewUserFolderResponse(folder),
		},
	)

}

// List ดึงรายการโฟลเดอร์ภายใต้ parent ที่ระบุ
//
// GET /api/folders
// หมายถึงดูโฟลเดอร์ที่ Root
//
// GET /api/folders?parent_id=5
// หมายถึงดูโฟลเดอร์ลูกภายใน Folder ID 5
func (h *FolderHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	var parentID *int64
	ownerUserID := claims.UserID

	parentIDParam := strings.TrimSpace(
		r.URL.Query().Get("parent_id"),
	)

	// ถ้ามี parent_id ให้แปลงและตรวจสอบ
	if parentIDParam != "" {
		parsedParentID, err := strconv.ParseInt(
			parentIDParam,
			10,
			64,
		)
		if err != nil || parsedParentID <= 0 {
			response.Error(
				w,
				http.StatusBadRequest,
				"รหัสโฟลเดอร์แม่ไม่ถูกต้อง",
			)
			return
		}

		// ตรวจว่า parent เป็นของ user คนนี้จริง
		parentFolder, err := h.findFolderForOwnerOrSharedViewer(
			r.Context(),
			parsedParentID,
			claims.UserID,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.Error(
					w,
					http.StatusNotFound,
					"ไม่พบโฟลเดอร์แม่",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"ไม่สามารถโหลดข้อมูลโฟลเดอร์ได้",
			)
			return
		}

		parentID = &parsedParentID
		ownerUserID = parentFolder.UserID
	}

	folders, err := h.folderRepo.FindAllByParentID(
		r.Context(),
		ownerUserID,
		parentID,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดรายการโฟลเดอร์ได้",
		)
		return
	}

	folderResponses := make(
		[]models.UserFolderResponse,
		0,
		len(folders),
	)

	for i := range folders {
		folderResponses = append(
			folderResponses,
			models.NewUserFolderResponse(&folders[i]),
		)
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":   "โหลดรายการโฟลเดอร์สำเร็จ",
			"folders":   folderResponses,
			"total":     len(folderResponses),
			"parent_id": parentID,
		},
	)
}

// GetByID ดึงรายละเอียดโฟลเดอร์ตาม ID
// GET /api/folders/{id}
func (h *FolderHandler) GetByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	folderID, err := strconv.ParseInt(
		r.PathValue("id"),
		10,
		64,
	)

	if err != nil || folderID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสโฟลเดอร์ไม่ถูกต้อง",
		)
		return
	}

	folder, err := h.findFolderForOwnerOrSharedViewer(
		r.Context(),
		folderID,
		claims.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบโฟลเดอร์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลโฟลเดอร์ได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "โหลดข้อมูลโฟลเดอร์สำเร็จ",
			"folder":  models.NewUserFolderResponse(folder),
		},
	)
}

// Rename สำหรับแก้ชื่อโฟลเดอร์
func (h *FolderHandler) Rename(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPatch {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method นี้",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)

		return
	}

	var req models.RenameFolderRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	// ตรวจสอบชื่อโฟลเดอร์
	if err := validateFolderName(req.FolderName); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	if len([]rune(req.FolderName)) > 255 {
		response.Error(
			w,
			http.StatusBadRequest,
			"ชื่อโฟลเดอร์ต้องไม่ยาวเกิน 255 อักษะ",
		)

		return
	}

	folderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || folderID < 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสโฟลเดอร์ไม่ถูกต้อง",
		)

		return
	}

	// ตรวจสอบว่า Fodler เป็นของตัวเองไหม กันยิง Request จาก Postmam
	userFolder, err := h.folderRepo.FindByIDAndUserID(
		r.Context(),
		folderID,
		claims.UserID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบโฟลเดอร์ที่ต้องการเปลี่ยนชือ",
			)

			return
		}
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดโฟลเดอร์ได้",
		)

		return
	}

	renamed, err := h.folderRepo.Rename(
		r.Context(),
		claims.UserID,
		userFolder.ID,
		req.FolderName,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถเปลี่ยนชื่อโฟลเดอร์ได้",
		)
		return
	}

	if !renamed {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบโฟลเดอร์ที่สามารถแก้ไขได้",
		)
		return
	}
	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":     "เปลี่ยนชื่อโฟลเดอร์สำเร็จ",
			"folder_id":   folderID,
			"folder_name": req.FolderName,
		},
	)
}

func (h *FolderHandler) MoveFile(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPatch {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method นี้",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)

		return
	}

	folderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || folderID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสโฟลเดอร์ไม่ถูกต้อง",
		)

		return
	}

	// จำกัดขนาด JSON request body
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		64*1024,
	)

	var req models.MoveFolderRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	// ตรวจสอบว่า Fodler เป็นของตัวเองไหม กันยิง Request จาก Postmam
	userFolder, err := h.folderRepo.FindByIDAndUserID(
		r.Context(),
		folderID,
		claims.UserID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบโฟลเดอร์ที่ต้องการเปลี่ยนชือ",
			)

			return
		}
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดโฟลเดอร์ได้",
		)

		return
	}

	// ถ้าจะย้ายเข้าโฟลเดอร์ ต้องตรวจว่าโฟลเดอร์มีอยู่จริง
	// เป็นของผู้ใช้ และยังไม่ถูก Soft Delete
	if req.FolderID != nil {
		if *req.FolderID <= 0 {
			response.Error(
				w,
				http.StatusBadRequest,
				"รหัสโฟลเดอร์ไม่ถูกต้อง",
			)
			return
		}
		_, err := h.folderRepo.FindByIDAndUserID(
			r.Context(),
			*req.FolderID,
			claims.UserID,
		)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.Error(
					w,
					http.StatusNotFound,
					"ไม่พบโฟลเดอร์ปลายทาง",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"ไม่สามารถตรวจสอบโฟลเดอร์ปลายทางได้",
			)
			return
		}
	}

	if err := h.folderRepo.MoveFolder(
		r.Context(),
		claims.UserID,
		userFolder.ID,
		req.FolderID,
	); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถย้ายโฟลเดอร์ได้",
		)
		return
	}
	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":   "ย้ายไฟล์สำเร็จ",
			"folder_id": folderID,
			"parent_id": req.FolderID,
		},
	)
}

func (h *FolderHandler) SoftDelete(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPatch {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method นี้",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	folderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil || folderID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสโฟลเดอร์ไม่ถูกต้อง",
		)

		return
	}

	// ตรวจสอบว่าโฟลเดอร์นี้เป็นของผู้ใช้จริง
	userFolder, err := h.folderRepo.FindByIDAndUserID(
		r.Context(),
		folderID,
		claims.UserID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบโฟลเดอร์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลโฟลเดอร์",
		)

		return
	}

	deletedFolders, deletedFiles, err := h.softDeleteFolderTree(
		r.Context(),
		claims.UserID,
		userFolder.ID,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถลบโฟลเดอร์ได้",
		)
		return
	}

	if deletedFolders == 0 {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบโฟลเดอร์ที่สามารถลบได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":         "ลบโฟลเดอร์สำเร็จ",
			"folder_id":       userFolder.ID,
			"deleted_folders": deletedFolders,
			"deleted_files":   deletedFiles,
		},
	)
}

func (h *FolderHandler) softDeleteFolderTree(
	ctx context.Context,
	userID int,
	folderID int64,
) (int, int, error) {
	deletedFolders := 0
	deletedFiles := 0

	childFolders, err := h.folderRepo.FindAllByParentID(
		ctx,
		userID,
		&folderID,
	)
	if err != nil {
		return 0, 0, err
	}

	for _, childFolder := range childFolders {
		childDeletedFolders, childDeletedFiles, err := h.softDeleteFolderTree(
			ctx,
			userID,
			childFolder.ID,
		)
		if err != nil {
			return 0, 0, err
		}

		deletedFolders += childDeletedFolders
		deletedFiles += childDeletedFiles
	}

	filesInFolder, err := h.fileRepo.FindAllByFolderID(
		ctx,
		userID,
		&folderID,
	)
	if err != nil {
		return 0, 0, err
	}

	for _, file := range filesInFolder {
		deleted, err := h.fileRepo.SoftDelete(
			ctx,
			file.ID,
			userID,
		)
		if err != nil {
			return 0, 0, err
		}

		if deleted {
			deletedFiles += 1
		}
	}

	deleted, err := h.folderRepo.SoftDelete(
		ctx,
		userID,
		folderID,
	)
	if err != nil {
		return 0, 0, err
	}

	if deleted {
		deletedFolders += 1
	}

	return deletedFolders, deletedFiles, nil
}

// validateFolderName ตรวจสอบชื่อโฟลเดอร์
func (h *FolderHandler) findFolderForOwnerOrSharedViewer(
	ctx context.Context,
	folderID int64,
	userID int,
) (*models.UserFolder, error) {
	folder, err := h.folderRepo.FindByIDAndUserID(
		ctx,
		folderID,
		userID,
	)
	if err == nil {
		return folder, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if h.sharedFolderRepo == nil {
		return nil, sql.ErrNoRows
	}

	permission, err := h.sharedFolderRepo.FindPermissionByFolderTreeAndUserID(
		ctx,
		folderID,
		userID,
	)
	if err != nil {
		return nil, err
	}

	if permission != models.SharedPermissionViewer && permission != models.SharedPermissionEditor {
		return nil, sql.ErrNoRows
	}

	return h.folderRepo.FindByID(ctx, folderID)
}

func validateFolderName(folderName string) error {
	if folderName == "" {
		return errors.New("กรุณากรอกชื่อโฟลเดอร์")
	}

	if len([]rune(folderName)) > 255 {
		return errors.New("ชื่อโฟลเดอร์ต้องไม่เกิน 255 ตัวอักษร")
	}

	if folderName == "." || folderName == ".." {
		return errors.New("ชื่อโฟลเดอร์ไม่ถูกต้อง")
	}

	// ไม่อนุญาต path separator เพราะเป็นชื่อโฟลเดอร์ Virtual
	if strings.ContainsAny(folderName, `/\`) {
		return errors.New("ชื่อโฟลเดอร์ห้ามมีเครื่องหมาย / หรือ \\")
	}

	// ไม่อนุญาต control character เช่น newline หรือ tab
	if strings.ContainsFunc(folderName, unicode.IsControl) {
		return errors.New("ชื่อโฟลเดอร์มีอักขระที่ไม่อนุญาต")
	}

	return nil
}

// isDuplicateEntry ตรวจว่าเป็น MySQL duplicate key error หรือไม่
func isDuplicateEntry(err error) bool {
	var mysqlErr *mysql.MySQLError

	return errors.As(err, &mysqlErr) &&
		mysqlErr.Number == 1062
}
