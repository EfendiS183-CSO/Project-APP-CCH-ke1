export function Section({ eyebrow, title, children, actions }) {
  return (
    <section className="workspace-section">
      <div className="section-head">
        <div>
          {eyebrow ? <p className="section-eyebrow">{eyebrow}</p> : null}
          <h2>{title}</h2>
        </div>
        {actions ? <div className="section-actions">{actions}</div> : null}
      </div>
      {children}
    </section>
  )
}