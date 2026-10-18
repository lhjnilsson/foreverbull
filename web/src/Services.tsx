import { type FormEvent, type ReactNode, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { EntryForm, ServiceFields, StatusText, TextControl } from "./fields";
import type { Service } from "./types";
import { postJson, readString, useJson, type Load } from "./useJson";

function Area({ list, detail }: { list: ReactNode; detail: ReactNode }) {
  return (
    <div className="split">
      <div className="list">{list}</div>
      <div className="detail">{detail}</div>
    </div>
  );
}

function latestStatus(service: Service): string | undefined {
  const statuses = service.statuses;
  if (!statuses || statuses.length === 0) return undefined;
  let latest = statuses[0];
  for (const item of statuses) {
    if ((item.OccurredAt ?? "") >= (latest?.OccurredAt ?? "")) latest = item;
  }
  return latest?.status;
}

function ServiceRow({ service, selected }: { service: Service; selected: boolean }) {
  const image = service.Image ?? "";
  const status = latestStatus(service);
  return (
    <Link
      className={selected ? "row selected" : "row"}
      to={{ pathname: "/services", search: `?${new URLSearchParams({ image }).toString()}` }}
      aria-current={selected ? "page" : undefined}
    >
      <span className="primary">{image}</span>
      <StatusText status={status} />
    </Link>
  );
}

function ServiceList({ load, selected }: { load: Load<Service[]>; selected: string | null }) {
  if (load.state === "ready" && Array.isArray(load.data)) {
    if (load.data.length === 0) return <p className="empty">Register a service.</p>;
    return (
      <>
        {load.data.map((service, index) => (
          <ServiceRow
            key={`${service.Image ?? ""}-${index}`}
            service={service}
            selected={service.Image !== undefined && service.Image === selected}
          />
        ))}
      </>
    );
  }
  if (load.state === "error" && load.message) return <p className="notice">{load.message}</p>;
  return null;
}

function ServiceDetail({ load }: { load: Load<Service> }) {
  if (load.state === "ready" && load.data) return <ServiceFields service={load.data} />;
  if (load.state === "error" && load.message) return <p className="notice">{load.message}</p>;
  return null;
}

function RegisterService({ onCreated }: { onCreated: (image: string) => void }) {
  const [image, setImage] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setError("");
    setPending(true);
    const result = await postJson("/api/services", { image: image.trim() });
    setPending(false);
    if (!result.ok) {
      setError(result.message);
      return;
    }
    const created = readString(result.data, "Image");
    if (!created) return;
    setImage("");
    onCreated(created);
  }

  return (
    <EntryForm error={error} pending={pending} submitLabel="Register this image." onSubmit={onSubmit}>
      <TextControl label="image" value={image} onChange={setImage} />
    </EntryForm>
  );
}

export function Services() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [revision, setRevision] = useState(0);
  const image = params.get("image");
  const list = useJson<Service[]>("/api/services", revision);
  // An image may contain slashes, so the id stays in the query.
  const detailUrl = image ? `/api/services?image=${encodeURIComponent(image)}` : null;
  const detail = useJson<Service>(detailUrl, revision);

  return (
    <Area
      list={<ServiceList load={list} selected={image} />}
      detail={
        <>
          <ServiceDetail load={detail} />
          <RegisterService
            onCreated={(created) => {
              setRevision((current) => current + 1);
              navigate({
                pathname: "/services",
                search: `?${new URLSearchParams({ image: created }).toString()}`,
              });
            }}
          />
        </>
      }
    />
  );
}
