import { type FormEvent, useState } from "react";
import { Link, Navigate, useNavigate, useSearchParams } from "react-router-dom";
import {
  Detail,
  Empty,
  Field,
  FormError,
  H2,
  ListPane,
  StatusText,
  buttonClass,
  fmtTime,
  inputClass,
  rowActive,
  rowClass,
} from "../components/fb";
import type { Service } from "../types";
import { postJson, readString, useJson } from "../useJson";

function servicePath(image: string): string {
  return `/services?${new URLSearchParams({ image }).toString()}`;
}

function rowsOf(data: Service[] | undefined): Service[] {
  return Array.isArray(data) ? data : [];
}

export function ServicesPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [revision, setRevision] = useState(0);
  const image = params.get("image");
  const list = useJson<Service[]>("/api/services", revision);
  const rows = list.state === "ready" ? rowsOf(list.data) : null;
  const first = rows?.find((service) => service.Image)?.Image;
  const known = image !== null && rows?.some((service) => service.Image === image) === true;
  const redirectTo = rows && rows.length > 0 && first && !known ? first : null;
  // An image may contain slashes, so the id stays in the query.
  const detailUrl = image ? `/api/services?${new URLSearchParams({ image }).toString()}` : null;
  const detail = useJson<Service>(detailUrl, revision);
  const fromList = rows?.find((service) => service.Image === image);
  const selected = detail.state === "ready" ? detail.data : detail.state === "loading" ? fromList : undefined;

  return (
    <>
      {redirectTo ? <Navigate to={servicePath(redirectTo)} replace /> : null}
      <ListPane
        title="Services"
        newLabel="New service"
        newTitle="Register this image."
        newForm={(close) => (
          <CreateService
            onDone={close}
            onCreated={(created) => {
              setRevision((current) => current + 1);
              navigate(servicePath(created));
            }}
          />
        )}
      >
        {list.state === "error" && list.message ? (
          <li className="px-4 py-6 text-muted-foreground">{list.message}</li>
        ) : null}
        {rows && rows.length === 0 ? <Empty text="Register a service." /> : null}
        {(rows ?? []).map((service) => (
          <li key={service.Image}>
            <Link
              to={service.Image ? servicePath(service.Image) : "/services"}
              className={`${rowClass} ${service.Image === image ? rowActive : ""}`}
            >
              <div className="truncate font-mono text-[13px]">{service.Image}</div>
              <StatusText s={service.statuses?.at(-1)?.status} />
            </Link>
          </li>
        ))}
      </ListPane>
      {redirectTo ? (
        <Detail k="none">
          <p className="text-muted-foreground">Select a service.</p>
        </Detail>
      ) : detail.state === "error" ? (
        <Detail k="error">
          <p className="text-muted-foreground">{detail.message || "not found"}</p>
        </Detail>
      ) : selected?.Image ? (
        <ServiceDetail service={selected} />
      ) : (
        <Detail k="none">
          <p className="text-muted-foreground">Select a service.</p>
        </Detail>
      )}
    </>
  );
}

function ServiceDetail({ service }: { service: Service }) {
  const names = service.algorithm?.functions
    ?.map((fn) => fn.name)
    .filter((name): name is string => Boolean(name))
    .join(", ");
  return (
    <Detail k={service.Image ?? "service"}>
      <h1 className="mb-4 break-all font-mono text-base">{service.Image}</h1>
      <dl>
        <Field label="Image">{service.Image}</Field>
        <Field label="File path">{service.algorithm?.filePath || "—"}</Field>
        <Field label="Functions">{names || "—"}</Field>
      </dl>
      <H2>Status history</H2>
      <ul>
        {(service.statuses ?? []).map((st, i) => (
          <li key={`${st.status ?? ""}-${st.OccurredAt ?? ""}-${i}`} className="grid grid-cols-[9rem_1fr] gap-4 border-b py-1.5">
            <StatusText s={st.status} />
            <span className="font-mono text-[13px] text-muted-foreground">
              {fmtTime(st.OccurredAt)}
              {st.error ? <span className="ml-3 text-status-error">{st.error}</span> : null}
            </span>
          </li>
        ))}
      </ul>
    </Detail>
  );
}

function CreateService({ onDone, onCreated }: { onDone: () => void; onCreated: (image: string) => void }) {
  const [image, setImage] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = image.trim();
    if (pending || !trimmed) return;
    setError("");
    setPending(true);
    const result = await postJson("/api/services", { image: trimmed });
    setPending(false);
    if (!result.ok) {
      setError(result.message);
      return;
    }
    const created = readString(result.data, "Image") ?? trimmed;
    setImage("");
    onCreated(created);
    onDone();
  }

  return (
    <form className="max-w-md" onSubmit={onSubmit}>
      <label className="mb-1 block text-muted-foreground" htmlFor="image">
        Image
      </label>
      <input
        id="image"
        className={inputClass}
        value={image}
        spellCheck={false}
        autoComplete="off"
        onChange={(event) => setImage(event.target.value)}
      />
      <button className={`${buttonClass} mt-4`} disabled={!image.trim() || pending}>
        Register this image.
      </button>
      <FormError message={error} />
    </form>
  );
}
