import React, { useMemo, useRef, useState } from 'react';
import { getUnitIcon } from '../../lib/gameAssets';

// MutaliskTimingChart renders the 1v1 TvZ Mutalisk-Turret timing as two
// horizontal lanes — Zerg (top) and Terran (bottom) — sharing one x-axis.
//
// Per lane: prerequisite icon → "building Xs" dotted span → "built" tick →
// red idle "gap" → unit icon → "building Xs" dotted span → final "built" tick
// (with the unit's completion mm:ss labelled).
//
// Between the two lanes a gap connector ties Mutalisk-built to Turret-built.
// A faint golden background under the connector marks the corpus expert
// sweet-spot (p25..p75 of turret-finish minus mutalisk-hatch). The connector
// itself is green when the actual gap falls inside the sweet spot, red
// otherwise.

const LEGEND_TOOLTIP =
  'Expert range comes from the cwal-dl 1v1 TvZ corpus (240-game match set). It is the p25-p75 interquartile of (turret_finished - mutalisk_finished); progamers aim for turrets to land just-in-time as mutas arrive.';

const formatTime = (sec) => {
  const v = Math.max(0, Math.floor(Number(sec) || 0));
  return `${Math.floor(v / 60)}:${String(v % 60).padStart(2, '0')}`;
};

const formatSigned = (n) => {
  const v = Number(n) || 0;
  if (v > 0) return `+${v}s`;
  if (v < 0) return `${v}s`;
  return '0s';
};

// Expert range geometry (relative to corpus median, in seconds):
//   invisible "perfect" zone: PERFECT_HALF_WIDTH on each side of the median.
//   golden flanking zones: GOLDEN_WIDTH wide on each side of the perfect zone.
// Within either zone = "within expert range" (green). Outside both = red.
const PERFECT_HALF_WIDTH = 4;     // 8s-wide invisible perfect rect
const GOLDEN_WIDTH       = 5.5;   // 5.5s golden rect on each flank
const EARLY_WASTE_THRESHOLD = 8;  // turret done < 8s after muta hatch (or before): minerals burned too early

const expertBoundaries = (median) => ({
  innerL: median - PERFECT_HALF_WIDTH,
  innerR: median + PERFECT_HALF_WIDTH,
  outerL: median - PERFECT_HALF_WIDTH - GOLDEN_WIDTH,
  outerR: median + PERFECT_HALF_WIDTH + GOLDEN_WIDTH,
});

const verdictForGap = (actual, median) => {
  if (!Number.isFinite(actual)) return null;
  if (actual < EARLY_WASTE_THRESHOLD) return 'Waste of early minerals';
  const { innerL, innerR, outerL, outerR } = expertBoundaries(median);
  if (actual >= innerL && actual <= innerR) return 'Perfect timing';
  if (actual >= outerL && actual <= outerR) return 'Within expert range';
  return 'Terran base at risk';
};

const pickLane = (group) => {
  const events = Array.isArray(group?.events) ? group.events : [];
  return events.map((e) => ({
    key: e.key,
    subject: e.subject,
    actual: Number(e.actual_second) || 0,
    found: Boolean(e.found),
    buildTime: Number(e.build_time_seconds) || 0,
    actualBuilt: Number(e.actual_built_second) || 0, // 0 = use actual + buildTime
  }));
};

const builtSecond = (e) =>
  Number(e.actualBuilt) > 0 ? Number(e.actualBuilt) : (e.actual + e.buildTime);

const NEUTRAL  = 'rgba(220, 224, 232, 0.95)';
const GOLDEN   = 'rgba(251, 191, 36, 0.85)';
const GOLDEN_BG = 'rgba(251, 191, 36, 0.20)';
const GREEN    = 'rgba(34, 197, 94, 0.95)';
const RED      = 'rgba(239, 68, 68, 0.85)';
const RED_BG   = 'rgba(239, 68, 68, 0.18)';
const LANE_Z_TINT = 'rgba(99, 175, 237, 0.06)';
const LANE_Z_BORDER = 'rgba(99, 175, 237, 0.20)';
const LANE_T_TINT = 'rgba(245, 158, 66, 0.06)';
const LANE_T_BORDER = 'rgba(245, 158, 66, 0.20)';

function MutaliskTimingChart({ zSide, tSide, summary }) {
  const wrapperRef = useRef(null);
  const [hover, setHover] = useState(null);

  const zEvents = useMemo(() => pickLane(zSide), [zSide]);
  const tEvents = useMemo(() => pickLane(tSide), [tSide]);

  const domain = useMemo(() => {
    const all = [...zEvents, ...tEvents];
    if (all.length === 0) return { min: 0, max: 600 };
    let min = Infinity;
    let max = -Infinity;
    all.forEach((e) => {
      if (e.found) {
        min = Math.min(min, e.actual);
        max = Math.max(max, builtSecond(e));
      }
    });
    if (!Number.isFinite(min)) min = 0;
    if (!Number.isFinite(max)) max = Math.max(600, min + 60);
    const span = Math.max(1, max - min);
    const pad = Math.max(8, Math.round(span * 0.06));
    return { min: Math.max(0, min - pad), max: max + pad };
  }, [zEvents, tEvents]);

  const chartWidth = 1040;
  const leftPadding = 80;
  const rightPadding = 36;
  const topPadding = 24;
  const bottomPadding = 36;
  const laneHeight = 88;
  const laneMid = 48; // y offset of lane horizontal line within the lane band
  const gapHeight = 100;
  const plotWidth = chartWidth - leftPadding - rightPadding;
  const chartHeight = topPadding + laneHeight + gapHeight + laneHeight + bottomPadding;
  const xAt = (sec) => {
    const span = Math.max(1, domain.max - domain.min);
    const bounded = Math.max(domain.min, Math.min(domain.max, Number(sec) || domain.min));
    return leftPadding + (((bounded - domain.min) / span) * plotWidth);
  };
  const tickCount = 7;
  const ticks = Array.from({ length: tickCount }).map((_, i) => {
    const span = Math.max(1, domain.max - domain.min);
    return Math.round(domain.min + ((span * i) / (tickCount - 1)));
  });

  const zLaneTop = topPadding;
  const tLaneTop = topPadding + laneHeight + gapHeight;

  const updateHover = (event, payload) => {
    if (!wrapperRef.current) return;
    const rect = wrapperRef.current.getBoundingClientRect();
    setHover({
      x: event.clientX - rect.left + 12,
      y: event.clientY - rect.top + 10,
      ...payload,
    });
  };

  // Render one event's icon + build span. The "built" final tick + completion
  // label is rendered separately for the LAST event in the lane (Mutalisk /
  // Missile Turret) since the user asked for differentiated treatment.
  const renderTriggerWithBuildSpan = (e, laneTop, opts = {}) => {
    if (!e.found) return null;
    const trigSec = e.actual;
    const doneSec = builtSecond(e);
    const iconURL = getUnitIcon(e.subject || e.key);
    const yMid = laneTop + laneMid;
    const xTrig = xAt(trigSec);
    const xDone = xAt(doneSec);

    return (
      <g key={`trig-${laneTop}-${e.key}`}>
        {/* Dotted build span from trigger icon to built tick */}
        <line
          x1={xTrig}
          y1={yMid}
          x2={xDone}
          y2={yMid}
          stroke={NEUTRAL}
          strokeWidth="1.5"
          strokeDasharray="3,4"
          opacity="0.85"
        />
        {/* "building Xs" centered on the dotted line. Skipped for the final
            event in each lane (where opts.label === false). */}
        {opts.label !== false && e.buildTime > 0 ? (
          <text
            x={(xTrig + xDone) / 2}
            y={yMid - 6}
            textAnchor="middle"
            fill="rgba(255,255,255,0.6)"
            fontSize="10"
          >
            building {e.buildTime}s
          </text>
        ) : null}
        {/* Trigger icon */}
        <g
          onMouseEnter={(ev) => updateHover(ev, {
            title: e.key,
            line1: `triggered at ${formatTime(trigSec)}`,
            line2: e.buildTime > 0 ? `${e.buildTime}s build → ${formatTime(doneSec)}` : '',
          })}
          onMouseMove={(ev) => updateHover(ev, {
            title: e.key,
            line1: `triggered at ${formatTime(trigSec)}`,
            line2: e.buildTime > 0 ? `${e.buildTime}s build → ${formatTime(doneSec)}` : '',
          })}
          onMouseLeave={() => setHover(null)}
        >
          {iconURL ? (
            <>
              <circle cx={xTrig} cy={yMid} r="14" fill="rgba(9,10,16,0.95)" stroke={NEUTRAL} strokeWidth="1.5" />
              <image
                href={iconURL}
                xlinkHref={iconURL}
                x={xTrig - 12}
                y={yMid - 12}
                width={24}
                height={24}
              />
            </>
          ) : (
            <circle cx={xTrig} cy={yMid} r="6" fill={NEUTRAL} stroke="rgba(9,10,16,0.95)" strokeWidth="1.25" />
          )}
        </g>
        {/* Trigger time label below icon */}
        <text
          x={xTrig}
          y={yMid + 26}
          textAnchor="middle"
          fill="rgba(255,255,255,0.7)"
          fontSize="10"
        >
          {formatTime(trigSec)}
        </text>
        {/* Built tick at completion */}
        <line
          x1={xDone}
          y1={yMid - 9}
          x2={xDone}
          y2={yMid + 9}
          stroke={NEUTRAL}
          strokeWidth="2"
        />
      </g>
    );
  };

  // Render the final-event "built" label. Position differs by lane:
  // mutaTimePosition='above' for Z (avoid overlap with downward gap connector),
  // 'below' for T (avoid bottom-axis ticks).
  const renderFinalBuiltLabel = (e, laneTop, position) => {
    if (!e.found) return null;
    const doneSec = builtSecond(e);
    const yMid = laneTop + laneMid;
    const yLabel = position === 'above' ? yMid - 14 : yMid + 24;
    return (
      <text
        x={xAt(doneSec)}
        y={yLabel}
        textAnchor="middle"
        fill={NEUTRAL}
        fontSize="11"
      >
        built {formatTime(doneSec)}
      </text>
    );
  };

  // Idle gap between previous event's "built" tick and the next event's
  // trigger icon. Rendered as a faint red band along the lane mid with a
  // small "gap Xs" label sitting well above the band so it never clashes
  // with the icons / build labels.
  const renderIdleGap = (prev, next, laneTop) => {
    if (!prev?.found || !next?.found) return null;
    const prevDone = builtSecond(prev);
    const nextStart = next.actual;
    if (nextStart <= prevDone) return null;
    const yMid = laneTop + laneMid;
    const x1 = xAt(prevDone);
    const x2 = xAt(nextStart);
    return (
      <g>
        <rect
          x={x1}
          y={yMid - 5}
          width={x2 - x1}
          height={10}
          fill={RED_BG}
          rx="2"
        />
        <line
          x1={x1}
          y1={yMid}
          x2={x2}
          y2={yMid}
          stroke={RED}
          strokeWidth="1.25"
          strokeDasharray="2,2"
          opacity="0.85"
        />
        <text
          x={(x1 + x2) / 2}
          y={yMid - 30}
          textAnchor="middle"
          fill={RED}
          fontSize="10"
        >
          gap {nextStart - prevDone}s
        </text>
        {/* Connector tick from label down to the band so it's visually
            anchored even when the gap is very narrow. */}
        <line
          x1={(x1 + x2) / 2}
          y1={yMid - 26}
          x2={(x1 + x2) / 2}
          y2={yMid - 6}
          stroke={RED}
          strokeWidth="0.75"
          opacity="0.5"
        />
      </g>
    );
  };

  // Gap connector + expert overlay
  let gapNode = null;
  let verdictNode = null;
  if (zEvents.length >= 2 && tEvents.length >= 2 && zEvents[1].found && tEvents[1].found) {
    const mutaBuilt = builtSecond(zEvents[1]);
    const turretBuilt = builtSecond(tEvents[1]);
    const actualGap = turretBuilt - mutaBuilt;
    const expertMedian = Number(summary?.expert_gap_seconds) || 0;
    const { outerR } = expertBoundaries(expertMedian);
    const verdict = verdictForGap(actualGap, expertMedian);
    const safe = actualGap >= EARLY_WASTE_THRESHOLD && actualGap <= outerR;
    const connectorColor = safe ? GREEN : RED;
    const verdictColor = safe ? GREEN : RED;
    const yTop = zLaneTop + laneMid;
    const yBot = tLaneTop + laneMid;
    const yMid = (yTop + yBot) / 2;
    const xMuta = xAt(mutaBuilt);
    const xTurret = xAt(turretBuilt);
    // Expert range — TWO golden rectangles flanking an invisible "perfect"
    // zone. Centered at the midpoint between the two marks so the band
    // reads as a visual reference for "this is the expected gap width";
    // the actual connector line either fits inside it (good) or extrudes
    // beyond it (bad). When actualGap is negative (turret first), the
    // band has no meaningful center, so we skip the overlays entirely
    // and let the connector + verdict speak for themselves.
    const midSecond = (mutaBuilt + turretBuilt) / 2;
    const halfPerfect = PERFECT_HALF_WIDTH;
    const halfOuter = PERFECT_HALF_WIDTH + GOLDEN_WIDTH;
    const showOverlays = actualGap >= EARLY_WASTE_THRESHOLD;
    const xOuterL = xAt(midSecond - halfOuter);
    const xInnerL = xAt(midSecond - halfPerfect);
    const xInnerR = xAt(midSecond + halfPerfect);
    const xOuterR = xAt(midSecond + halfOuter);
    gapNode = (
      <g>
        {showOverlays ? (
          <>
            {/* Left golden rect (outerL → innerL) — sits behind the connector */}
            <rect
              x={Math.min(xOuterL, xInnerL)}
              y={yMid - 6}
              width={Math.abs(xInnerL - xOuterL)}
              height={12}
              fill={GOLDEN_BG}
              stroke={GOLDEN}
              strokeWidth="1"
              rx="2"
            />
            {/* Right golden rect (innerR → outerR) — sits behind the connector */}
            <rect
              x={Math.min(xInnerR, xOuterR)}
              y={yMid - 6}
              width={Math.abs(xOuterR - xInnerR)}
              height={12}
              fill={GOLDEN_BG}
              stroke={GOLDEN}
              strokeWidth="1"
              rx="2"
            />
          </>
        ) : null}
        {/* Vertical guides from each lane's "built" tick to the gap mid */}
        <line x1={xMuta} y1={yTop + 9} x2={xMuta} y2={yMid} stroke={connectorColor} strokeWidth="1" strokeDasharray="2,3" opacity="0.7" />
        <line x1={xTurret} y1={yMid} x2={xTurret} y2={yBot - 9} stroke={connectorColor} strokeWidth="1" strokeDasharray="2,3" opacity="0.7" />
        {/* Horizontal connector + end caps */}
        <line x1={xMuta} y1={yMid} x2={xTurret} y2={yMid} stroke={connectorColor} strokeWidth="2.5" />
        <line x1={xMuta} y1={yMid - 5} x2={xMuta} y2={yMid + 5} stroke={connectorColor} strokeWidth="2" />
        <line x1={xTurret} y1={yMid - 5} x2={xTurret} y2={yMid + 5} stroke={connectorColor} strokeWidth="2" />
        {/* Gap label above connector */}
        <text x={(xMuta + xTurret) / 2} y={yMid - 10} textAnchor="middle" fill={connectorColor} fontSize="11">
          gap {formatSigned(actualGap)}
        </text>
        {/* Verdict line just under the gap overlays */}
        {verdict ? (
          <text x={(xMuta + xTurret) / 2} y={yMid + 18} textAnchor="middle" fill={verdictColor} fontSize="10">
            {verdict}
          </text>
        ) : null}
      </g>
    );
  }

  // Build event lookups
  const zPrereq = zEvents[0];
  const zUnit   = zEvents[1];
  const tPrereq = tEvents[0];
  const tUnit   = tEvents[1];

  return (
    <div className="workflow-card timing-chart-card">
      <div className="workflow-first-unit-title" style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
        <span><strong>Mutalisk-Turret timing</strong></span>
        <span className="workflow-first-unit-title-slash">·</span>
        <span style={{ color: 'rgba(255,255,255,0.7)' }}>
          {zSide?.name ? `${zSide.name} (Z)` : 'Zerg'} vs {tSide?.name ? `${tSide.name} (T)` : 'Terran'}
        </span>
        <span
          style={{ marginLeft: 'auto', color: GOLDEN, cursor: 'help', fontSize: '11px' }}
          title={LEGEND_TOOLTIP}
        >
          * Expert range
        </span>
      </div>
      <div ref={wrapperRef} className="workflow-timing-chart-wrap">
        <svg className="workflow-timing-scatter" viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="xMinYMin meet">
          {/* Bottom ticks (shared time axis) */}
          {ticks.map((sec) => (
            <g key={`tk-${sec}`}>
              <line
                x1={xAt(sec)}
                y1={topPadding - 4}
                x2={xAt(sec)}
                y2={chartHeight - bottomPadding + 4}
                stroke="rgba(255,255,255,0.06)"
                strokeWidth="1"
              />
              <text
                x={xAt(sec)}
                y={chartHeight - bottomPadding + 18}
                textAnchor="middle"
                fill="rgba(255,255,255,0.55)"
                fontSize="10"
              >
                {formatTime(sec)}
              </text>
            </g>
          ))}
          {/* Lane backgrounds */}
          <rect
            x={leftPadding - 4}
            y={zLaneTop + 2}
            width={plotWidth + 8}
            height={laneHeight - 4}
            fill={LANE_Z_TINT}
            stroke={LANE_Z_BORDER}
            rx="6"
          />
          <rect
            x={leftPadding - 4}
            y={tLaneTop + 2}
            width={plotWidth + 8}
            height={laneHeight - 4}
            fill={LANE_T_TINT}
            stroke={LANE_T_BORDER}
            rx="6"
          />
          {/* Lane labels */}
          <text x={leftPadding - 12} y={zLaneTop + laneMid + 4} textAnchor="end" fill="rgba(255,255,255,0.85)" fontSize="12">Zerg</text>
          <text x={leftPadding - 12} y={zLaneTop + laneMid + 18} textAnchor="end" fill="rgba(255,255,255,0.5)" fontSize="9">{zSide?.name || ''}</text>
          <text x={leftPadding - 12} y={tLaneTop + laneMid + 4} textAnchor="end" fill="rgba(255,255,255,0.85)" fontSize="12">Terran</text>
          <text x={leftPadding - 12} y={tLaneTop + laneMid + 18} textAnchor="end" fill="rgba(255,255,255,0.5)" fontSize="9">{tSide?.name || ''}</text>
          {/* Lane base lines */}
          <line x1={leftPadding} y1={zLaneTop + laneMid} x2={chartWidth - rightPadding} y2={zLaneTop + laneMid} stroke="rgba(255,255,255,0.06)" strokeWidth="1" />
          <line x1={leftPadding} y1={tLaneTop + laneMid} x2={chartWidth - rightPadding} y2={tLaneTop + laneMid} stroke="rgba(255,255,255,0.06)" strokeWidth="1" />

          {/* Z lane events */}
          {zPrereq ? renderTriggerWithBuildSpan(zPrereq, zLaneTop) : null}
          {zPrereq && zUnit ? renderIdleGap(zPrereq, zUnit, zLaneTop) : null}
          {zUnit ? renderTriggerWithBuildSpan(zUnit, zLaneTop, { label: false }) : null}
          {zUnit ? renderFinalBuiltLabel(zUnit, zLaneTop, 'above') : null}

          {/* T lane events */}
          {tPrereq ? renderTriggerWithBuildSpan(tPrereq, tLaneTop) : null}
          {tPrereq && tUnit ? renderIdleGap(tPrereq, tUnit, tLaneTop) : null}
          {tUnit ? renderTriggerWithBuildSpan(tUnit, tLaneTop, { label: false }) : null}
          {tUnit ? renderFinalBuiltLabel(tUnit, tLaneTop, 'below') : null}

          {gapNode}
          {verdictNode}
        </svg>
        {hover ? (
          <div
            className="workflow-timing-tooltip"
            style={{ left: `${hover.x}px`, top: `${hover.y}px` }}
          >
            <div><strong>{hover.title}</strong></div>
            {hover.line1 ? <div>{hover.line1}</div> : null}
            {hover.line2 ? <div>{hover.line2}</div> : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export default MutaliskTimingChart;
