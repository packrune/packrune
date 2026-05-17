// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

import type { CSSProperties } from "react";

/**
 * AuroraBackground paints three large soft-edged colored blobs that drift
 * very slowly behind the rest of the UI. Three because two looks like a
 * gradient and four starts to look like a screensaver. Uses CSS animations
 * (not motion) because we don't want this to fight reduced-motion settings —
 * the animation duration is also tuned so it's barely-perceptible at rest.
 *
 * The colors come from theme variables, so every theme automatically gets a
 * tasteful aurora without component code changing.
 */
export function AuroraBackground() {
  const style: CSSProperties = {
    position: "fixed",
    inset: 0,
    zIndex: -1,
    overflow: "hidden",
    pointerEvents: "none",
  };
  return (
    <div style={style} aria-hidden>
      <Blob xPct={15} yPct={20} colorVar="--aurora-1" durationS={28} />
      <Blob xPct={75} yPct={15} colorVar="--aurora-2" durationS={36} delayS={-12} />
      <Blob xPct={50} yPct={80} colorVar="--aurora-3" durationS={44} delayS={-6} />
      <Veil />
      <style>{`
        @keyframes pkr-aurora-drift {
          0%   { transform: translate3d(0,0,0) scale(1); }
          50%  { transform: translate3d(4vw,-3vh,0) scale(1.06); }
          100% { transform: translate3d(0,0,0) scale(1); }
        }
      `}</style>
    </div>
  );
}

function Blob({
  xPct,
  yPct,
  colorVar,
  durationS,
  delayS = 0,
}: {
  xPct: number;
  yPct: number;
  colorVar: string;
  durationS: number;
  delayS?: number;
}) {
  const style: CSSProperties = {
    position: "absolute",
    left: `${xPct}%`,
    top: `${yPct}%`,
    width: "55vmax",
    height: "55vmax",
    transform: "translate(-50%, -50%)",
    borderRadius: "9999px",
    filter: "blur(110px)",
    background: `radial-gradient(circle at center, rgb(var(${colorVar}) / 0.55) 0%, rgb(var(${colorVar}) / 0) 60%)`,
    animation: `pkr-aurora-drift ${durationS}s ease-in-out ${delayS}s infinite`,
  };
  return <div style={style} />;
}

function Veil() {
  // A subtle grain + dimmer veil so the aurora doesn't blow out text contrast.
  const style: CSSProperties = {
    position: "absolute",
    inset: 0,
    background:
      "radial-gradient(ellipse at top, transparent 0%, var(--bg-base) 70%), linear-gradient(180deg, transparent, var(--bg-base) 90%)",
    opacity: 0.7,
  };
  return <div style={style} />;
}
