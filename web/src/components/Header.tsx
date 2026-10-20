import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { useApp } from "../auth/AuthProvider";

export function Header({ children }: { children?: ReactNode }) {
  const { user, loading, accounts, accountName, canSignOut, signIn, signOut } = useApp();
  return (
    <header className="topbar">
      <Link to="/" className="brand" aria-label="Orapa, accueil">
        <svg viewBox="0 0 32 32" aria-hidden>
          <path d="M4 26 16 6l12 20z" fill="#3e7bfa" />
          <path d="M16 6l12 20H16z" fill="#2a5fd6" />
          <path d="M2 16h9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
        <span>Orapa</span>
      </Link>
      <div className="topbar-mid">{children}</div>
      <div className="account">
        {accounts && !loading &&
          (user ? (
            <>
              <span className="account-name">{user.name}</span>
              {canSignOut && (
                <button className="btn btn-small btn-quiet" onClick={signOut}>
                  Se déconnecter
                </button>
              )}
            </>
          ) : (
            <button className="btn btn-small" onClick={() => signIn()}>
              {accountName ? `Se connecter avec ${accountName}` : "Se connecter"}
            </button>
          ))}
      </div>
    </header>
  );
}
