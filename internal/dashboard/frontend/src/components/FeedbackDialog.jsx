import React, { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useT, getLocale } from '../lib/i18nContext';
import { api } from '../api';
import { MAX_SCREENSHOTS, prepareScreenshot } from '../lib/feedbackScreenshots';

const DISCORD_PROFILE_URL = 'https://discord.com/users/516408972652183576';
const GITHUB_ISSUE_URL = 'https://github.com/marianogappa/screpdb/issues/new/choose';

function FeedbackDialog({ onClose }) {
  const t = useT();
  const [message, setMessage] = useState('');
  const [contact, setContact] = useState('');
  const [shots, setShots] = useState([]);
  const [status, setStatus] = useState('editing');
  const [error, setError] = useState('');
  const fileInput = useRef(null);
  const sending = status === 'sending';

  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape' && !sending) onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose, sending]);

  useEffect(() => () => shots.forEach((shot) => URL.revokeObjectURL(shot.preview)), [shots]);

  const addFiles = useCallback(async (files) => {
    const images = [...files].filter((file) => file.type.startsWith('image/'));
    if (images.length === 0) return;
    const room = MAX_SCREENSHOTS - shots.length;
    if (images.length > room) setError(t('feedback.error.tooManyScreenshots', { max: MAX_SCREENSHOTS }));
    const prepared = [];
    for (const file of images.slice(0, Math.max(room, 0))) {
      try {
        prepared.push(await prepareScreenshot(file));
      } catch {
        setError(t('feedback.error.unreadableImage'));
      }
    }
    if (prepared.length > 0) setShots((current) => [...current, ...prepared].slice(0, MAX_SCREENSHOTS));
  }, [shots.length, t]);

  useEffect(() => {
    if (status !== 'editing') return undefined;
    const onPaste = (e) => {
      const files = [...(e.clipboardData?.files || [])];
      if (files.some((file) => file.type.startsWith('image/'))) {
        e.preventDefault();
        addFiles(files);
      }
    };
    window.addEventListener('paste', onPaste);
    return () => window.removeEventListener('paste', onPaste);
  }, [addFiles, status]);

  const removeShot = (index) => {
    setShots((current) => current.filter((_, i) => i !== index));
  };

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
    });
    if (result === 'sent') {
      setStatus('sent');
      return;
    }
    setStatus('editing');
    setError(t(`feedback.error.${result}`));
  };

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
                    <button type="button" onClick={() => removeShot(i)} disabled={sending} aria-label={t('feedback.removeScreenshot')}>×</button>
                  </div>
                ))}
                {shots.length < MAX_SCREENSHOTS ? (
                  <button type="button" className="feedback-add-shot" onClick={() => fileInput.current?.click()} disabled={sending}>
                    {t('feedback.addScreenshots')}
                  </button>
                ) : null}
              </div>
              <p className="feedback-hint">{t('feedback.screenshotHint')}</p>
              <input
                ref={fileInput}
                type="file"
                accept="image/*"
                multiple
                hidden
                onChange={(e) => { addFiles(e.target.files); e.target.value = ''; }}
              />
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
