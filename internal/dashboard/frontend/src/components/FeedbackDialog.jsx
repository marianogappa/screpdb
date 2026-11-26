import React, { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useT, getLocale } from '../lib/i18nContext';
import { api } from '../api';
import {
  MAX_ATTACHMENT_BYTES,
  MAX_ATTACHMENT_MB,
  MAX_REPLAYS,
  MAX_SCREENSHOTS,
  attachmentBytes,
  isImageFile,
  isReplayFile,
  prepareReplay,
  prepareScreenshot,
} from '../lib/feedbackAttachments';

const DISCORD_PROFILE_URL = 'https://discord.com/users/516408972652183576';
const GITHUB_ISSUE_URL = 'https://github.com/marianogappa/screpdb/issues/new/choose';

const formatKB = (bytes) => `${Math.max(1, Math.round(bytes / 1024))} KB`;

function FeedbackDialog({ onClose }) {
  const t = useT();
  const [message, setMessage] = useState('');
  const [contact, setContact] = useState('');
  const [shots, setShots] = useState([]);
  const [replays, setReplays] = useState([]);
  const [status, setStatus] = useState('editing');
  const [error, setError] = useState('');
  const shotInput = useRef(null);
  const replayInput = useRef(null);
  const sending = status === 'sending';

  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape' && !sending) onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose, sending]);

  useEffect(() => () => shots.forEach((shot) => URL.revokeObjectURL(shot.preview)), [shots]);

  const addFiles = useCallback(async (fileList) => {
    const files = [...fileList];
    const images = files.filter(isImageFile);
    const reps = files.filter(isReplayFile);
    if (images.length + reps.length < files.length) setError(t('feedback.error.unsupportedFile'));
    let used = attachmentBytes(shots, replays);
    const fits = (size) => {
      if (used + size > MAX_ATTACHMENT_BYTES) {
        setError(t('feedback.error.tooLarge', { mb: MAX_ATTACHMENT_MB }));
        return false;
      }
      used += size;
      return true;
    };

    const newShots = [];
    if (shots.length + images.length > MAX_SCREENSHOTS) setError(t('feedback.error.tooManyScreenshots', { max: MAX_SCREENSHOTS }));
    for (const file of images.slice(0, Math.max(MAX_SCREENSHOTS - shots.length, 0))) {
      try {
        const shot = await prepareScreenshot(file);
        if (fits(shot.size)) newShots.push(shot);
      } catch {
        setError(t('feedback.error.unreadableImage'));
      }
    }
    const newReplays = [];
    if (replays.length + reps.length > MAX_REPLAYS) setError(t('feedback.error.tooManyReplays', { max: MAX_REPLAYS }));
    for (const file of reps.slice(0, Math.max(MAX_REPLAYS - replays.length, 0))) {
      if (!fits(file.size)) continue;
      try {
        newReplays.push(await prepareReplay(file));
      } catch {
        used -= file.size;
        setError(t('feedback.error.unreadableFile'));
      }
    }
    if (newShots.length > 0) setShots((current) => [...current, ...newShots]);
    if (newReplays.length > 0) setReplays((current) => [...current, ...newReplays]);
  }, [shots, replays, t]);

  useEffect(() => {
    if (status !== 'editing') return undefined;
    const onPaste = (e) => {
      const files = [...(e.clipboardData?.files || [])];
      if (files.some((file) => isImageFile(file) || isReplayFile(file))) {
        e.preventDefault();
        addFiles(files);
      }
    };
    window.addEventListener('paste', onPaste);
    return () => window.removeEventListener('paste', onPaste);
  }, [addFiles, status]);

  const send = async (e) => {
    e.preventDefault();
    if (!message.trim() || sending) return;
    setStatus('sending');
    setError('');
    const result = await api.sendFeedback({
      message,
      contact,
      language: getLocale(),
      screenshots: shots.map((shot) => shot.base64),
      replays: replays.map((replay) => ({ name: replay.name, data: replay.base64 })),
    });
    if (result === 'sent') {
      setStatus('sent');
      return;
    }
    setStatus('editing');
    setError(t(`feedback.error.${result}`));
  };

  const pickerButton = (input, label) => (
    <button type="button" className="feedback-add-shot" onClick={() => input.current?.click()} disabled={sending}>
      {label}
    </button>
  );

  return (
    <div className="modal-overlay" onClick={sending ? undefined : onClose}>
      <div
        className="modal-content feedback-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="feedback-dialog-title"
        onClick={(e) => e.stopPropagation()}
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => { e.preventDefault(); if (status === 'editing') addFiles(e.dataTransfer.files); }}
      >
        <div className="modal-header">
          <h2 id="feedback-dialog-title">{t('feedback.title')}</h2>
          <button type="button" className="btn-close" onClick={onClose} disabled={sending} aria-label={t('feedback.close')}>×</button>
        </div>
        {status === 'sent' ? (
          <div className="feedback-body">
            <p className="feedback-sent">{t('feedback.sent')}</p>
            <div className="feedback-actions">
              <button type="button" className="feedback-primary" onClick={onClose}>{t('feedback.close')}</button>
            </div>
          </div>
        ) : (
          <form className="feedback-body" onSubmit={send}>
            <p className="feedback-privacy">{t('feedback.privacy')}</p>
            <textarea
              className="feedback-message"
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder={t('feedback.messagePlaceholder')}
              aria-label={t('feedback.messageLabel')}
              maxLength={4000}
              rows={6}
              autoFocus
              disabled={sending}
            />
            <div className="feedback-shots">
              <div className="feedback-shots-row">
                {shots.map((shot, i) => (
                  <div className="feedback-shot" key={shot.preview}>
                    <img src={shot.preview} alt={t('feedback.screenshotAlt', { n: i + 1 })} />
                    <button type="button" onClick={() => setShots((current) => current.filter((_, j) => j !== i))} disabled={sending} aria-label={t('feedback.removeScreenshot')}>×</button>
                  </div>
                ))}
                {replays.map((replay, i) => (
                  <div className="feedback-replay" key={`${replay.name}-${i}`}>
                    <span className="feedback-replay-name" title={replay.name}>{replay.name}</span>
                    <span className="feedback-replay-size">{formatKB(replay.size)}</span>
                    <button type="button" onClick={() => setReplays((current) => current.filter((_, j) => j !== i))} disabled={sending} aria-label={t('feedback.removeReplay')}>×</button>
                  </div>
                ))}
                {shots.length < MAX_SCREENSHOTS ? pickerButton(shotInput, t('feedback.addScreenshots')) : null}
                {replays.length < MAX_REPLAYS ? pickerButton(replayInput, t('feedback.addReplays')) : null}
              </div>
              <p className="feedback-hint">
                {t('feedback.attachmentHint', { screenshots: MAX_SCREENSHOTS, replays: MAX_REPLAYS, mb: MAX_ATTACHMENT_MB })}
              </p>
              <input ref={shotInput} type="file" accept="image/*" multiple hidden onChange={(e) => { addFiles(e.target.files); e.target.value = ''; }} />
              <input ref={replayInput} type="file" accept=".rep" multiple hidden onChange={(e) => { addFiles(e.target.files); e.target.value = ''; }} />
            </div>
            <label className="feedback-contact">
              <span>{t('feedback.contactLabel')}</span>
              <input
                type="text"
                value={contact}
                onChange={(e) => setContact(e.target.value)}
                placeholder={t('feedback.contactPlaceholder')}
                maxLength={200}
                disabled={sending}
              />
            </label>
            {error ? <p className="feedback-error" role="alert">{error}</p> : null}
            <div className="feedback-actions">
              <span className="feedback-alternatives">
                <a href={DISCORD_PROFILE_URL} target="_blank" rel="noopener noreferrer">{t('feedback.discord')}</a>
                <span aria-hidden="true"> · </span>
                <a href={GITHUB_ISSUE_URL} target="_blank" rel="noopener noreferrer">{t('feedback.github')}</a>
              </span>
              <button type="submit" className="feedback-primary" disabled={sending || !message.trim()}>
                {sending ? t('feedback.sending') : t('feedback.send')}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}

export default function FeedbackLink() {
  const t = useT();
  const [open, setOpen] = useState(false);
  const close = useCallback(() => setOpen(false), []);
  return (
    <>
      <button type="button" className="footer-feedback-link" onClick={() => setOpen(true)}>
        {t('footer.reportIssue')}
      </button>
      {open ? createPortal(<FeedbackDialog onClose={close} />, document.body) : null}
    </>
  );
}
