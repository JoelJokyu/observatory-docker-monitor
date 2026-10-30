import { useEffect, useRef } from 'react';
import { CircleHelp, X } from 'lucide-react';

export default function AboutModal({ onClose }: { onClose: () => void }) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    const trigger = document.activeElement;
    dialog.showModal();
    return () => {
      if (dialog.open) dialog.close();
      if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus();
    };
  }, []);
  return (
    <dialog
      ref={dialogRef}
      className="workspace-modal"
      aria-labelledby="workspace-about-title"
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section className="workspace-about">
        <div className="about-heading">
          <span>
            <CircleHelp size={17} />
            ABOUT OBSERVATORY
          </span>
          <button
            autoFocus
            className="icon-button"
            onClick={onClose}
            aria-label="Close workspace information"
          >
            <X size={18} />
          </button>
        </div>
        <h2 id="workspace-about-title">Your local monitoring workspace</h2>
        <p>
          Observatory collects from one Docker Engine and retains seven days of resource
          measurements. Select a container to inspect its resource, I/O, and Docker-backed log
          snapshots.
        </p>
        <dl className="about-definitions">
          <div>
            <dt>CPU</dt>
            <dd>100% represents one fully used core.</dd>
          </div>
          <div>
            <dt>NET I/O & BLOCK I/O</dt>
            <dd>Cumulative bytes; history charts show rates per second.</dd>
          </div>
          <div>
            <dt>PIDs</dt>
            <dd>Includes processes and threads.</dd>
          </div>
          <div>
            <dt>Missing readings</dt>
            <dd>Stay unknown rather than appearing as zero.</dd>
          </div>
        </dl>
        <p className="about-note">
          Log availability follows Docker’s retention. This local development build has no
          authentication and is intended for localhost use.
        </p>
      </section>
    </dialog>
  );
}
