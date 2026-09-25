package campaigns

import (
	"fmt"
	"text/template"

	"github.com/a-h/templ"
)

// surface_accent_onclick.go builds the Surface Accents card's click/change
// handlers as inline IIFEs in onclick/onchange attributes, never a <script>
// tag: boot.js sets htmx.config.allowScriptTags=false, so htmx deletes any
// <script> in a swapped-in fragment, and this card is reached by a boosted
// sidebar swap (.ai/conventions.md, HTMX swap-safety). Same pattern as
// internal/plugins/foundry_vtt/onclick_handlers.go. Only server-controlled
// values (IDs, the CSRF token, preset colors) are interpolated; the custom
// color is read from the input when the handler runs.

// jsStr returns a single-quoted JS string literal for embedding in an inline
// attribute handler (which templ delimits with double quotes).
func jsStr(s string) string {
	return "'" + template.JSEscapeString(s) + "'"
}

// inlineHandler wraps a JS body in a ComponentScript with an empty Function
// (no <script> tag emitted) so the body renders directly into the attribute.
func inlineHandler(name, jsBody string) templ.ComponentScript {
	return templ.ComponentScript{Name: name, Function: "", Call: jsBody}
}

// applySurfaceAccentJS is the shared body for the preset/reset buttons and
// the custom picker: local CSS-variable preview first (instant recolor of the
// sample card), then persisted via the existing accent endpoint. colorExpr is
// a JS expression yielding the chosen color ("" clears the slot).
func applySurfaceAccentJS(campaignID, csrfToken string, slot int, colorExpr string) string {
	return fmt.Sprintf(
		`(function(){`+
			`var color=%s;`+
			`var prop='--color-accent-surface-%d';`+
			`if(color){document.documentElement.style.setProperty(prop,color);}`+
			`else{document.documentElement.style.removeProperty(prop);}`+
			`if(window.htmx){window.htmx.ajax('PUT','/campaigns/'+%s+'/accent-color',{`+
			`values:{accent_color:color,slot:'%d'},`+
			`headers:{'X-CSRF-Token':%s},`+
			`swap:'none'`+
			`});}`+
			`})()`,
		colorExpr, slot, jsStr(campaignID), slot, jsStr(csrfToken),
	)
}

// surfaceAccentApplyOnClick returns the onclick handler for a preset swatch
// (or the reset button, with color=""): the slot and color are both known at
// render time, so they're baked in as literals.
func surfaceAccentApplyOnClick(campaignID, csrfToken string, slot int, color string) templ.ComponentScript {
	body := applySurfaceAccentJS(campaignID, csrfToken, slot, jsStr(color))
	return inlineHandler(fmt.Sprintf("surfaceAccentApply_%d_%s", slot, color), body)
}

// surfaceAccentCustomOnChange returns the onchange handler for the custom
// color <input>: the color isn't known until the user picks one, so it reads
// the input's own current value at fire time via its id.
func surfaceAccentCustomOnChange(campaignID, csrfToken string, slot int, inputID string) templ.ComponentScript {
	body := applySurfaceAccentJS(campaignID, csrfToken, slot,
		fmt.Sprintf(`document.getElementById(%s).value`, jsStr(inputID)))
	return inlineHandler(fmt.Sprintf("surfaceAccentCustom_%d", slot), body)
}
