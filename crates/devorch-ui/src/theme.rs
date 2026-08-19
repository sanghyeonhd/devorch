//! Visual style.
//!
//! Devorch's UI shows verdicts — passed, rejected, merged, denied — so color
//! carries meaning here rather than decoration. Every status color is defined
//! once, and each is paired with a text label at the call site: a reviewer with
//! colorblindness must be able to read a comparison table.

use egui::{Color32, Context, Rounding, Stroke, Visuals};

/// Something verified and accepted.
pub const PASS: Color32 = Color32::from_rgb(64, 160, 96);
/// Something rejected on evidence.
pub const FAIL: Color32 = Color32::from_rgb(200, 72, 72);
/// Something awaiting a human decision.
pub const WAITING: Color32 = Color32::from_rgb(206, 154, 60);
/// Something in progress.
pub const ACTIVE: Color32 = Color32::from_rgb(72, 132, 208);
/// Secondary text.
pub const MUTED: Color32 = Color32::from_rgb(140, 144, 152);
/// The winning candidate.
pub const WINNER: Color32 = Color32::from_rgb(120, 190, 130);

/// Apply the Devorch look to `ctx`.
pub fn install(ctx: &Context) {
    let mut visuals = Visuals::dark();
    visuals.panel_fill = Color32::from_rgb(24, 26, 30);
    visuals.window_fill = Color32::from_rgb(28, 30, 35);
    visuals.extreme_bg_color = Color32::from_rgb(18, 20, 23);
    visuals.widgets.noninteractive.bg_stroke = Stroke::new(1.0_f32, Color32::from_rgb(48, 52, 58));
    visuals.widgets.inactive.rounding = Rounding::same(4.0);
    visuals.widgets.hovered.rounding = Rounding::same(4.0);
    visuals.widgets.active.rounding = Rounding::same(4.0);
    visuals.selection.bg_fill = Color32::from_rgb(48, 84, 132);
    ctx.set_visuals(visuals);

    let mut style = (*ctx.style()).clone();
    style.spacing.item_spacing = egui::vec2(8.0, 6.0);
    style.spacing.button_padding = egui::vec2(10.0, 5.0);
    ctx.set_style(style);
}

/// The color for a pass/fail verdict.
pub fn verdict_color(passed: bool) -> Color32 {
    if passed {
        PASS
    } else {
        FAIL
    }
}

/// The label for a pass/fail verdict.
///
/// Always drawn next to [`verdict_color`] so the meaning does not rely on color
/// alone.
pub fn verdict_label(passed: bool) -> &'static str {
    if passed {
        "passed"
    } else {
        "rejected"
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn verdicts_carry_both_a_color_and_a_word() {
        assert_eq!(verdict_color(true), PASS);
        assert_eq!(verdict_color(false), FAIL);
        assert_eq!(verdict_label(true), "passed");
        assert_eq!(verdict_label(false), "rejected");
    }

    #[test]
    fn status_colors_are_distinguishable_from_each_other() {
        let colors = [PASS, FAIL, WAITING, ACTIVE, MUTED, WINNER];
        for (i, a) in colors.iter().enumerate() {
            for b in colors.iter().skip(i + 1) {
                assert_ne!(a, b, "two status colors are identical");
            }
        }
    }
}
