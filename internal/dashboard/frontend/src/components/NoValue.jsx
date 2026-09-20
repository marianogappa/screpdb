import React from 'react';
import { t } from '../lib/i18nContext';

// One placeholder for every value we do not have, so a gap never passes for a
// measurement. A real zero is a number and renders as one; this renders as the
// absence of a number, and says why on hover.
//
// The glyph is a plain hyphen on purpose: an em dash is banned across this
// codebase (see lib/noEmDash.test.js) and a blank cell reads as a layout bug.
export default function NoValue({ reason }) {
  return (
    <span className="no-value" title={reason || t('common.notKnown')} aria-label={reason || t('common.notKnown')}>
      -
    </span>
  );
}
