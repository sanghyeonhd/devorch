//! The 3D agent constellation.
//!
//! A mission is a graph: one goal, several agents working in isolation, and one
//! result. A table shows the verdicts but not the *shape* — that two candidates
//! ran in parallel while two waited, that three converged on a merge and one was
//! cut. This view draws that shape.
//!
//! It is drawn with an orthographic projection onto egui's 2D painter rather
//! than a GPU scene graph. That is a deliberate limit: the value here is
//! spatial legibility, not photorealism, and keeping it on the painter means
//! the whole product still builds without a shader toolchain.

use std::f32::consts::TAU;

use devorch_mission::{CandidateRecord, MissionRecord};
use devorch_protocol::AgentKind;
use egui::{Color32, Pos2, Rect, Sense, Stroke, Ui, Vec2};

use crate::theme;

/// A point in the constellation's own space.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Point3 {
    pub x: f32,
    pub y: f32,
    pub z: f32,
}

impl Point3 {
    /// A point at `(x, y, z)`.
    pub const fn new(x: f32, y: f32, z: f32) -> Self {
        Self { x, y, z }
    }

    /// Rotate around the vertical axis by `angle` radians.
    pub fn rotate_y(self, angle: f32) -> Self {
        let (sin, cos) = angle.sin_cos();
        Self {
            x: self.x * cos + self.z * sin,
            y: self.y,
            z: -self.x * sin + self.z * cos,
        }
    }

    /// Tilt around the horizontal axis by `angle` radians.
    pub fn rotate_x(self, angle: f32) -> Self {
        let (sin, cos) = angle.sin_cos();
        Self {
            x: self.x,
            y: self.y * cos - self.z * sin,
            z: self.y * sin + self.z * cos,
        }
    }
}

/// How the constellation is being viewed.
#[derive(Debug, Clone, Copy)]
pub struct Camera {
    /// Rotation around the vertical axis, in radians.
    pub yaw: f32,
    /// Tilt, in radians.
    pub pitch: f32,
    /// Scale, in pixels per unit.
    pub zoom: f32,
}

impl Default for Camera {
    fn default() -> Self {
        // A slight offset from straight-on so depth reads immediately, without
        // the operator having to drag before the layout makes sense.
        Self {
            yaw: 0.6,
            pitch: 0.35,
            zoom: 110.0,
        }
    }
}

impl Camera {
    /// How far the pitch may tilt, in radians.
    ///
    /// Clamped short of vertical: looking straight down collapses the layout
    /// into a line and there is no way to tell what happened.
    pub const MAX_PITCH: f32 = 1.2;

    /// Apply a drag, in pixels.
    pub fn drag(&mut self, delta: Vec2) {
        self.yaw = (self.yaw + delta.x * 0.01).rem_euclid(TAU);
        self.pitch = (self.pitch + delta.y * 0.01).clamp(-Self::MAX_PITCH, Self::MAX_PITCH);
    }

    /// Apply a zoom step.
    pub fn zoom_by(&mut self, factor: f32) {
        self.zoom = (self.zoom * factor).clamp(40.0, 400.0);
    }

    /// Project a point to screen space, returning its depth as well.
    ///
    /// Depth is returned so callers can sort: without it, a node behind the
    /// centre would paint over one in front and the layout would read wrong.
    pub fn project(&self, point: Point3, center: Pos2) -> (Pos2, f32) {
        let rotated = point.rotate_y(self.yaw).rotate_x(self.pitch);
        // Orthographic with a gentle depth scale: enough parallax to read the
        // arrangement, not enough to distort the comparison.
        let depth_scale = 1.0 + rotated.z * 0.12;
        let screen = Pos2::new(
            center.x + rotated.x * self.zoom * depth_scale,
            center.y - rotated.y * self.zoom * depth_scale,
        );
        (screen, rotated.z)
    }
}

/// What a node in the constellation represents.
#[derive(Debug, Clone, PartialEq)]
pub enum NodeKind {
    /// The mission goal.
    Goal,
    /// One agent's candidate.
    Candidate {
        agent: AgentKind,
        passed: bool,
        winner: bool,
    },
    /// The merged result.
    Merge { reached: bool },
}

/// One node, positioned in the constellation.
#[derive(Debug, Clone)]
pub struct Node {
    pub position: Point3,
    pub kind: NodeKind,
    pub label: String,
    /// Line under the label: the evidence that decided this node.
    pub detail: String,
}

impl Node {
    /// The color this node paints in.
    pub fn color(&self) -> Color32 {
        match &self.kind {
            NodeKind::Goal => theme::ACTIVE,
            NodeKind::Candidate { winner: true, .. } => theme::WINNER,
            NodeKind::Candidate { passed, .. } => theme::verdict_color(*passed),
            NodeKind::Merge { reached: true } => theme::PASS,
            NodeKind::Merge { reached: false } => theme::MUTED,
        }
    }

    /// The radius this node paints at, before depth scaling.
    pub fn radius(&self) -> f32 {
        match self.kind {
            NodeKind::Goal => 13.0,
            NodeKind::Merge { .. } => 11.0,
            NodeKind::Candidate { winner: true, .. } => 12.0,
            NodeKind::Candidate { .. } => 9.0,
        }
    }
}

/// A connection between two nodes.
#[derive(Debug, Clone)]
pub struct Edge {
    pub from: usize,
    pub to: usize,
    /// False for a candidate that was cut: drawn, but dashed and dim, because
    /// what was rejected is part of the story.
    pub carried: bool,
}

/// A mission laid out in three dimensions.
#[derive(Debug, Clone, Default)]
pub struct Constellation {
    pub nodes: Vec<Node>,
    pub edges: Vec<Edge>,
}

impl Constellation {
    /// Lay out a mission.
    ///
    /// The goal sits above, candidates form a ring at the waist, and the merge
    /// sits below. Candidates are spread evenly around the ring so a
    /// four-candidate mission reads as symmetric and a one-candidate mission
    /// reads as a straight line — the shape itself says how much comparison the
    /// router decided was worth paying for.
    pub fn from_mission(mission: &MissionRecord) -> Self {
        let mut nodes = vec![Node {
            position: Point3::new(0.0, 1.5, 0.0),
            kind: NodeKind::Goal,
            label: "goal".to_string(),
            detail: truncate(&mission.goal, 48),
        }];
        let mut edges = Vec::new();

        let count = mission.candidates.len().max(1);
        for (index, candidate) in mission.candidates.iter().enumerate() {
            let angle = TAU * index as f32 / count as f32;
            let winner = mission.winner == Some(candidate.agent);

            nodes.push(Node {
                position: Point3::new(angle.cos() * 1.6, 0.0, angle.sin() * 1.6),
                kind: NodeKind::Candidate {
                    agent: candidate.agent,
                    passed: candidate.passed,
                    winner,
                },
                label: candidate.agent.as_str().to_string(),
                detail: candidate_detail(candidate),
            });

            let node_index = nodes.len() - 1;
            edges.push(Edge {
                from: 0,
                to: node_index,
                carried: true,
            });
        }

        let merged = mission.merge_commit.is_some();
        nodes.push(Node {
            position: Point3::new(0.0, -1.5, 0.0),
            kind: NodeKind::Merge { reached: merged },
            label: if merged { "merged" } else { "not merged" }.to_string(),
            detail: mission
                .merge_commit
                .as_ref()
                .map(|c| c[..12.min(c.len())].to_string())
                .unwrap_or_else(|| {
                    mission
                        .failure
                        .clone()
                        .unwrap_or_else(|| "no candidate passed".into())
                }),
        });

        let merge_index = nodes.len() - 1;
        for (index, candidate) in mission.candidates.iter().enumerate() {
            edges.push(Edge {
                from: index + 1,
                to: merge_index,
                // Only the winner's work reached the merge. A rejected
                // candidate's edge is still drawn — dimmed — because the
                // comparison is the point.
                carried: merged && mission.winner == Some(candidate.agent),
            });
        }

        Self { nodes, edges }
    }
}

/// One line of evidence for a candidate.
fn candidate_detail(candidate: &CandidateRecord) -> String {
    if let Some(rejection) = &candidate.rejection {
        return truncate(rejection, 40);
    }
    format!(
        "{} files · {} churn",
        candidate.touched_paths, candidate.churn
    )
}

/// Shorten `text` to `limit` characters, on a character boundary.
fn truncate(text: &str, limit: usize) -> String {
    if text.chars().count() <= limit {
        return text.to_string();
    }
    let kept: String = text.chars().take(limit.saturating_sub(1)).collect();
    format!("{}…", kept.trim_end())
}

/// Draw the constellation, returning the camera after any interaction.
pub fn show(ui: &mut Ui, constellation: &Constellation, camera: &mut Camera) {
    let available = ui.available_size();
    let (response, painter) = ui.allocate_painter(available, Sense::drag());
    let rect = response.rect;
    let center = rect.center();

    painter.rect_filled(rect, 6.0, Color32::from_rgb(16, 18, 21));

    if response.dragged() {
        camera.drag(response.drag_delta());
    }
    if response.hovered() {
        let scroll = ui.input(|i| i.smooth_scroll_delta.y);
        if scroll.abs() > 0.0 {
            camera.zoom_by(1.0 + scroll * 0.002);
        }
    }

    // Project once, then sort by depth so nearer nodes paint last.
    let projected: Vec<(Pos2, f32)> = constellation
        .nodes
        .iter()
        .map(|node| camera.project(node.position, center))
        .collect();

    for edge in &constellation.edges {
        let (Some((from, _)), Some((to, _))) = (projected.get(edge.from), projected.get(edge.to))
        else {
            continue;
        };
        let color = if edge.carried {
            theme::WINNER.gamma_multiply(0.8)
        } else {
            Color32::from_rgb(60, 64, 72)
        };
        let width: f32 = if edge.carried { 2.0 } else { 1.0 };
        painter.line_segment([*from, *to], Stroke::new(width, color));
    }

    let mut order: Vec<usize> = (0..constellation.nodes.len()).collect();
    order.sort_by(|a, b| {
        projected[*a]
            .1
            .partial_cmp(&projected[*b].1)
            .unwrap_or(std::cmp::Ordering::Equal)
    });

    for index in order {
        let node = &constellation.nodes[index];
        let (position, depth) = projected[index];

        // Nearer nodes are larger and brighter, which is what makes the ring
        // read as a ring rather than a row.
        let scale = 1.0 + depth * 0.18;
        let radius = node.radius() * scale.clamp(0.6, 1.5);
        let dim = ((depth + 2.0) / 4.0).clamp(0.45, 1.0);

        painter.circle_filled(position, radius, node.color().gamma_multiply(dim));
        painter.circle_stroke(
            position,
            radius,
            Stroke::new(1.0_f32, Color32::from_rgb(20, 22, 26)),
        );

        painter.text(
            position + Vec2::new(0.0, radius + 10.0),
            egui::Align2::CENTER_CENTER,
            &node.label,
            egui::FontId::proportional(12.0 * scale.clamp(0.8, 1.2)),
            Color32::from_gray((200.0 * dim) as u8),
        );
        painter.text(
            position + Vec2::new(0.0, radius + 24.0),
            egui::Align2::CENTER_CENTER,
            &node.detail,
            egui::FontId::proportional(10.0),
            theme::MUTED.gamma_multiply(dim),
        );
    }

    // The interaction is not discoverable on its own.
    painter.text(
        Rect::from_min_size(rect.min, rect.size()).left_bottom() + Vec2::new(10.0, -10.0),
        egui::Align2::LEFT_BOTTOM,
        "drag to rotate · scroll to zoom",
        egui::FontId::proportional(10.0),
        theme::MUTED,
    );
}

#[cfg(test)]
mod tests {
    use super::*;
    use devorch_mission::MissionStatus;
    use std::path::Path;

    fn candidate(agent: AgentKind, passed: bool) -> CandidateRecord {
        CandidateRecord {
            agent,
            workspace: agent.as_str().into(),
            passed,
            rejection: (!passed).then(|| "tests failed".to_string()),
            touched_paths: 1,
            churn: 2,
            untracked: Vec::new(),
            duration_ms: 1000,
            agent_claimed_success: true,
        }
    }

    fn mission(candidates: Vec<CandidateRecord>, winner: Option<AgentKind>) -> MissionRecord {
        let mut record = MissionRecord::new("fix add()", Path::new("/repo"), "abc123");
        record.candidates = candidates;
        if let Some(winner) = winner {
            record.succeed(Some(winner), Some("def4567890ab"));
        } else {
            record.fail("no candidate passed the gate");
        }
        record
    }

    #[test]
    fn a_mission_lays_out_as_goal_candidates_and_merge() {
        let record = mission(
            vec![
                candidate(AgentKind::Codex, true),
                candidate(AgentKind::Grok, false),
            ],
            Some(AgentKind::Codex),
        );
        let constellation = Constellation::from_mission(&record);

        assert_eq!(constellation.nodes.len(), 4, "goal + 2 candidates + merge");
        assert!(matches!(constellation.nodes[0].kind, NodeKind::Goal));
        assert!(matches!(
            constellation.nodes.last().unwrap().kind,
            NodeKind::Merge { reached: true }
        ));
    }

    #[test]
    fn only_the_winners_edge_carries_to_the_merge() {
        let record = mission(
            vec![
                candidate(AgentKind::Codex, true),
                candidate(AgentKind::Grok, false),
                candidate(AgentKind::Claude, true),
            ],
            Some(AgentKind::Codex),
        );
        let constellation = Constellation::from_mission(&record);

        let merge_index = constellation.nodes.len() - 1;
        let carried: Vec<_> = constellation
            .edges
            .iter()
            .filter(|e| e.to == merge_index && e.carried)
            .collect();

        assert_eq!(carried.len(), 1, "exactly one candidate reached the merge");
    }

    #[test]
    fn a_failed_mission_still_draws_every_candidate() {
        // What was rejected is part of the story, so nothing is hidden.
        let record = mission(
            vec![
                candidate(AgentKind::Codex, false),
                candidate(AgentKind::Grok, false),
            ],
            None,
        );
        let constellation = Constellation::from_mission(&record);

        assert_eq!(constellation.nodes.len(), 4);
        assert!(matches!(
            constellation.nodes.last().unwrap().kind,
            NodeKind::Merge { reached: false }
        ));
        assert!(
            constellation.edges.iter().all(|e| e.to != 3 || !e.carried),
            "nothing reached the merge"
        );
        assert_eq!(record.status, MissionStatus::Failed);
    }

    #[test]
    fn candidates_are_spread_evenly_around_the_ring() {
        let record = mission(
            AgentKind::ALL.iter().map(|a| candidate(*a, true)).collect(),
            Some(AgentKind::Codex),
        );
        let constellation = Constellation::from_mission(&record);

        let positions: Vec<_> = constellation.nodes[1..5]
            .iter()
            .map(|n| n.position)
            .collect();

        // All at the same height and the same distance from the axis.
        for position in &positions {
            assert!(position.y.abs() < f32::EPSILON);
            let radius = (position.x.powi(2) + position.z.powi(2)).sqrt();
            assert!((radius - 1.6).abs() < 0.001, "radius was {radius}");
        }
        // And distinct from each other.
        for (i, a) in positions.iter().enumerate() {
            for b in positions.iter().skip(i + 1) {
                assert!((a.x - b.x).abs() > 0.001 || (a.z - b.z).abs() > 0.001);
            }
        }
    }

    #[test]
    fn rotation_preserves_distance_from_the_origin() {
        let point = Point3::new(1.6, 0.0, 0.0);
        let rotated = point.rotate_y(0.9).rotate_x(0.4);
        let length = |p: Point3| (p.x * p.x + p.y * p.y + p.z * p.z).sqrt();

        assert!((length(point) - length(rotated)).abs() < 0.001);
    }

    #[test]
    fn the_camera_clamps_pitch_short_of_vertical() {
        // Looking straight down collapses the ring into a line.
        let mut camera = Camera::default();
        for _ in 0..500 {
            camera.drag(Vec2::new(0.0, 10.0));
        }
        assert!(camera.pitch <= Camera::MAX_PITCH);

        for _ in 0..1000 {
            camera.drag(Vec2::new(0.0, -10.0));
        }
        assert!(camera.pitch >= -Camera::MAX_PITCH);
    }

    #[test]
    fn yaw_wraps_rather_than_growing_without_bound() {
        let mut camera = Camera::default();
        for _ in 0..2000 {
            camera.drag(Vec2::new(10.0, 0.0));
        }
        assert!(
            camera.yaw >= 0.0 && camera.yaw < TAU,
            "yaw was {}",
            camera.yaw
        );
    }

    #[test]
    fn zoom_is_bounded_in_both_directions() {
        let mut camera = Camera::default();
        for _ in 0..100 {
            camera.zoom_by(1.5);
        }
        assert!(camera.zoom <= 400.0);

        for _ in 0..200 {
            camera.zoom_by(0.5);
        }
        assert!(camera.zoom >= 40.0);
    }

    #[test]
    fn projection_reports_depth_so_nodes_can_be_sorted() {
        let camera = Camera::default();
        let center = Pos2::new(400.0, 300.0);

        let (_, near) = camera.project(Point3::new(0.0, 0.0, 1.6), center);
        let (_, far) = camera.project(Point3::new(0.0, 0.0, -1.6), center);
        assert!(near != far, "depth must distinguish front from back");
    }

    #[test]
    fn long_goals_are_truncated_rather_than_overflowing_the_node() {
        let long = "a".repeat(200);
        let shortened = truncate(&long, 48);
        assert_eq!(shortened.chars().count(), 48);
        assert!(shortened.ends_with('…'));
    }

    #[test]
    fn a_rejected_candidates_reason_is_its_detail_line() {
        let record = mission(vec![candidate(AgentKind::Grok, false)], None);
        let constellation = Constellation::from_mission(&record);
        assert_eq!(constellation.nodes[1].detail, "tests failed");
    }
}
