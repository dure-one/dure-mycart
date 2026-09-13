package handlers

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/webutil"
)

// The two marks a shop uploads. Each holds the name of a file in lc_uploads,
// and each is written by the endpoints below rather than through the raw
// key/value PATCH fallback — see groupOnlySettingKeys.
const (
	settingKeyBrandingLogo    = "branding_logo"
	settingKeyBrandingFavicon = "branding_favicon"
)

// UploadBrandingLogo stores a logo and points the branding group at it.
//
// @Summary      Upload the shop logo
// @Description  Store a PNG or JPEG as the logo the storefront draws in its header
// @Tags         Settings
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        document formData file true "Logo file (PNG or JPEG)"
// @Success      200 {object} webutil.HTTPResponse{result=models.Branding} "Logo uploaded"
// @Failure      400 {object} webutil.HTTPResponse "Invalid file format"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/branding/logo [post]
func UploadBrandingLogo(c fiber.Ctx) error {
	return uploadBrandingFile(c, settingKeyBrandingLogo, "Logo")
}

// UploadBrandingFavicon stores the icon the browser shows for the shop.
//
// @Summary      Upload the shop favicon
// @Description  Store a PNG or JPEG as the icon the storefront hands the browser
// @Tags         Settings
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        document formData file true "Favicon file (PNG or JPEG)"
// @Success      200 {object} webutil.HTTPResponse{result=models.Branding} "Favicon uploaded"
// @Failure      400 {object} webutil.HTTPResponse "Invalid file format"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/branding/favicon [post]
func UploadBrandingFavicon(c fiber.Ctx) error {
	return uploadBrandingFile(c, settingKeyBrandingFavicon, "Favicon")
}

// DeleteBrandingLogo puts the storefront back on the mark it was built with.
//
// @Summary      Delete the shop logo
// @Description  Clear the logo and remove the stored file
// @Tags         Settings
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse{result=models.Branding} "Logo deleted"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/branding/logo [delete]
func DeleteBrandingLogo(c fiber.Ctx) error {
	return deleteBrandingFile(c, settingKeyBrandingLogo, "Logo")
}

// DeleteBrandingFavicon removes the uploaded icon and the file behind it.
//
// @Summary      Delete the shop favicon
// @Description  Clear the favicon and remove the stored file
// @Tags         Settings
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse{result=models.Branding} "Favicon deleted"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/branding/favicon [delete]
func DeleteBrandingFavicon(c fiber.Ctx) error {
	return deleteBrandingFile(c, settingKeyBrandingFavicon, "Favicon")
}

// uploadBrandingFile stores an uploaded image under a name of the store's own
// choosing and writes that name into one of the branding keys.
//
// Only PNG and JPEG are accepted, on the same terms as product images: the
// storefront serves whatever lands in lc_uploads from its own origin, and an
// SVG would be a script the shop hands the browser on the shop's own behalf.
//
// The file the operator uploaded before is removed once the new one is in
// place. Without that, uploading a logo twice leaves the first one in
// lc_uploads with nothing pointing at it, and the directory is copied and
// backed up as a whole.
func uploadBrandingFile(c fiber.Ctx, key, label string) error {
	db := queries.DB()
	log := logging.New()

	file, err := c.FormFile("document")
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	// Trust the file's bytes, not the Content-Type the client declared.
	mimeType, err := sniffMIMEType(file)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, "cannot read uploaded file")
	}
	if !validateImageMIME(mimeType) {
		return webutil.StatusBadRequest(c, "file format not supported")
	}
	if !slices.Contains(validImageExtensions, normalizeExt(file.Filename)) {
		return webutil.StatusBadRequest(c, "file extension not supported")
	}

	_, _, fileName := generateFileName(file.Filename)
	filePath := filepath.Join(dirUploads, fileName)

	if err := saveFile(file, filePath); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// A stored file that does not decode is one the storefront would draw as a
	// broken picture, so it is refused before anything points at it.
	if _, err := imaging.Open(filePath); err != nil {
		log.ErrorStack(err)
		_ = os.Remove(filePath)
		return webutil.StatusBadRequest(c, "file is not a valid image")
	}

	branding, err := queries.GetSettingByGroup[models.Branding](c.Context(), db)
	if err != nil {
		log.ErrorStack(err)
		_ = os.Remove(filePath)
		return webutil.StatusInternalServerError(c)
	}

	previous := brandingValue(branding, key)
	setBrandingValue(branding, key, fileName)

	if err := db.UpdateSettingByGroup(c.Context(), branding); err != nil {
		log.ErrorStack(err)
		_ = os.Remove(filePath)
		return webutil.StatusInternalServerError(c)
	}

	removeUpload(previous, fileName)

	return webutil.Response(c, fiber.StatusOK, label+" uploaded", branding)
}

// deleteBrandingFile clears one of the two marks and removes its file. The
// storefront falls back to the mark it was built with, which is what a shop
// that has uploaded nothing has always shown.
func deleteBrandingFile(c fiber.Ctx, key, label string) error {
	db := queries.DB()
	log := logging.New()

	branding, err := queries.GetSettingByGroup[models.Branding](c.Context(), db)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	previous := brandingValue(branding, key)
	setBrandingValue(branding, key, "")

	if err := db.UpdateSettingByGroup(c.Context(), branding); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	removeUpload(previous, "")

	return webutil.Response(c, fiber.StatusOK, label+" deleted", branding)
}

// brandingValue reads one of the two marks by the setting key that names it.
func brandingValue(branding *models.Branding, key string) string {
	if key == settingKeyBrandingFavicon {
		return branding.Favicon
	}
	return branding.Logo
}

// setBrandingValue writes one of the two marks by the setting key that names it.
func setBrandingValue(branding *models.Branding, key, value string) {
	if key == settingKeyBrandingFavicon {
		branding.Favicon = value
		return
	}
	branding.Logo = value
}

// removeUpload deletes a file this shop stored, unless it is the one just
// written or there is nothing to delete. Base is taken so that a value that
// somehow carries a path still cannot name a file outside the uploads
// directory — the same rule the branding group is validated under.
func removeUpload(name, keep string) {
	if name == "" || name == keep {
		return
	}
	_ = os.Remove(filepath.Join(dirUploads, filepath.Base(name)))
}
