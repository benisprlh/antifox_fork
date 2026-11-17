// AntiFox fox emblem: transparent PNG sized through className for both themes.
export function AntifoxLogo({ className }: { className?: string }) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img src="/logo.png" alt="AntiFox" className={className} />
  );
}
