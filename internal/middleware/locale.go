package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/pkg/i18n"
)

// Locale resolves each request's language and stores a localizer that
// pkg/response uses for msg. Preference order: ?lang= query, Accept-Language
// header, then the translator's default locale.
func Locale(tr *i18n.Translator) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("localizer", tr.Localizer(c.Query("lang"), c.GetHeader("Accept-Language")))
		c.Next()
	}
}
