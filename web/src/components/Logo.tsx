// Логотип под тему: оба варианта в разметке, видимый выбирает CSS по data-theme - без мигания при смене.
export function Logo() {
  return (
    <span className="logo">
      <img className="logo-dark" src="/favicon.png" alt="" />
      <img className="logo-light" src="/favicon-light.png" alt="" />
    </span>
  );
}
