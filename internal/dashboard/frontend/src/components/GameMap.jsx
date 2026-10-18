import React, { forwardRef } from 'react';

// GameMap is the one map frame every map view uses (Summary, Game Events, engine snapshots), so a fix to
// sizing or zoom lands everywhere at once.
//
// view is the part of the map to show, in map pixels ({x, y, w, h}); null shows the whole map. The frame
// takes the view's aspect ratio and is never taller than maxHeight, a CSS length expression such as
// "240px" or "100vh - 28px".
//
// Two layer kinds:
// - mapLayers: SVG drawn in map pixels (sprites, regions), zooming with the view.
// - children: drawn in a box that always covers the whole map, wherever the view is, so the existing
//   overlays that place things in percent of the map (polygons in a 0..100 viewBox, icons at left/top %)
//   keep working unchanged and zoom with the view too.
//
// detail ({url, x, y, w, h} in map pixels) is a sharper terrain image for part of the map, drawn over the
// base image: the whole-map image is downsampled, which blurs close views.
const GameMap = forwardRef(function GameMap({
  mapUrl,
  mapWidth,
  mapHeight,
  view = null,
  maxHeight = '70vh',
  className = '',
  alt = '',
  mapLayers = null,
  detail = null,
  children = null,
  style = undefined,
}, ref) {
  const width = Number(mapWidth) || 0;
  const height = Number(mapHeight) || 0;
  if (!width || !height) {
    // Without the map's size the image's own proportions are all there is: no view, no map layers.
    return (
      <div ref={ref} className={`game-map game-map--natural ${className}`.trim()} style={style}>
        <img src={mapUrl} alt={alt} className="game-map-img" style={{ maxHeight: `calc(${maxHeight})` }} />
        {children ? <div className="game-map-whole" style={{ inset: 0 }}>{children}</div> : null}
      </div>
    );
  }
  const v = view || { x: 0, y: 0, w: width, h: height };
  const aspect = v.w / v.h;
  const wholeMapBox = {
    left: `${(-v.x / v.w) * 100}%`,
    top: `${(-v.y / v.h) * 100}%`,
    width: `${(width / v.w) * 100}%`,
    height: `${(height / v.h) * 100}%`,
  };
  return (
    <div
      ref={ref}
      className={`game-map ${className}`.trim()}
      // A fixed width with a max-width, not min(100%, ...): inside a shrink-to-fit parent (the Summary
      // thumbnail button) a percentage width resolves to nothing and the map vanishes.
      style={{ aspectRatio: `${v.w} / ${v.h}`, width: `calc((${maxHeight}) * ${aspect})`, maxWidth: '100%', ...style }}
    >
      <svg className="game-map-svg" viewBox={`${v.x} ${v.y} ${v.w} ${v.h}`} preserveAspectRatio="none" role="img" aria-label={alt}>
        <image href={mapUrl} x="0" y="0" width={width} height={height} preserveAspectRatio="none" />
        {detail ? <image href={detail.url} x={detail.x} y={detail.y} width={detail.w} height={detail.h} preserveAspectRatio="none" /> : null}
        {mapLayers}
      </svg>
      {children ? <div className="game-map-whole" style={wholeMapBox}>{children}</div> : null}
    </div>
  );
});

export default GameMap;
