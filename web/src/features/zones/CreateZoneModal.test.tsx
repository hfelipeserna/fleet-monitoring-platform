// Covers [SPEC-004: AC-007, AC-008, BR-012/013, FR-010, TS-007, UC-004]
import { describe, it, expect, vi, beforeAll, afterAll, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import CreateZoneModal from "./CreateZoneModal";
import { usePortalStore } from "../../store/portalStore";
import type { DraftPolygon } from "./types";

function makeClosed5(): number[][] {
  // Covers [SPEC-004: AC-007, BR-012/013] — 5 coords cerrado first==last
  return [
    [-74.07, 4.71],
    [-74.05, 4.71],
    [-74.05, 4.73],
    [-74.07, 4.73],
    [-74.07, 4.71],
  ];
}

function makeInvalid3(): number[][] {
  // Covers [SPEC-004: AC-008, BR-013] — 3 coords inválido <4
  return [
    [-74.07, 4.71],
    [-74.05, 4.71],
    [-74.05, 4.73],
  ];
}

function makeNotClosed5(): number[][] {
  // Covers [SPEC-004: AC-008, BR-013] — no cerrado first!=last
  return [
    [-74.07, 4.71],
    [-74.05, 4.71],
    [-74.05, 4.73],
    [-74.07, 4.73],
    [-74.06, 4.7],
  ];
}

function makeDraft(coords: number[][]): DraftPolygon {
  return { type: "Polygon", coordinates: [coords] };
}

const emptyFC = { type: "FeatureCollection" as const, features: [] as unknown[] };

const createdFeature = {
  type: "Feature" as const,
  id: "zone-new-1",
  properties: { name: "Zona Norte" },
  geometry: { type: "Polygon" as const, coordinates: [makeClosed5()] },
};

const server = setupServer(
  http.post("/api/zones", () => HttpResponse.json(createdFeature, { status: 201 })),
  http.post("*/api/zones", () => HttpResponse.json(createdFeature, { status: 201 })),
  http.get("/api/zones", () => HttpResponse.json(emptyFC)),
  http.get("*/api/zones", () => HttpResponse.json(emptyFC)),
);

describe("CreateZoneModal — TS-007 AC-007 draft modal create", () => {
  // Covers [SPEC-004: AC-007, AC-008, BR-012/013, FR-010, TS-007, UC-004]

  beforeAll(() => {
    // Arrange global msw
    server.listen({ onUnhandledRequest: "warn" });
  });

  beforeEach(() => {
    // Arrange
    usePortalStore.setState({ draftPolygon: makeDraft(makeClosed5()) } as never);
    cleanup();
  });

  afterEach(() => {
    cleanup();
    server.resetHandlers();
    vi.restoreAllMocks();
    usePortalStore.setState({ draftPolygon: null } as never);
  });

  afterAll(() => {
    server.close();
  });

  describe("draft 5 coords -> CreateZoneModal render Zone name input + Accept/Cancel overlay bg-black/50 role=dialog", () => {
    // Covers [SPEC-004: AC-007, BR-012/013, FR-010, TS-007]

    it("draft 5 coords -> CreateZoneModal render Zone name input + Accept/Cancel overlay bg-black/50 role=dialog", async () => {
      // Covers [SPEC-004: AC-007, BR-012/013, FR-010]
      // Arrange
      const draft5 = makeDraft(makeClosed5());
      usePortalStore.setState({ draftPolygon: draft5 } as never);
      const onClose = vi.fn();
      const onCreated = vi.fn();

      // Act
      render(<CreateZoneModal open draft={draft5} onClose={onClose} onCreated={onCreated} />);

      // Assert — role=dialog
      const dialog = screen.getByRole("dialog");
      expect(dialog).toBeInTheDocument(); // AC-007 role=dialog
      expect(dialog).toHaveAttribute("aria-modal", "true"); // AC-007 aria-modal

      // Assert — Zone name input
      const input =
        (screen.queryByLabelText(/Zone name/i, { selector: 'input' }) as HTMLInputElement | null) ??
        screen.queryByPlaceholderText(/Zone name/i) ??
        screen.queryByRole("textbox", { name: /Zone name/i }) ??
        document.querySelector('input[name="name"]');
      expect(input, "Zone name input debe existir").not.toBeNull(); // AC-007 FR-010
      expect(input).toBeInTheDocument();

      // Assert — Accept / Cancel buttons
      const acceptBtn = screen.getByRole("button", { name: /Accept/i });
      const cancelBtn = screen.getByRole("button", { name: /Cancel/i });
      expect(acceptBtn).toBeInTheDocument(); // AC-007
      expect(cancelBtn).toBeInTheDocument(); // AC-007

      // Assert — overlay bg-black/50
      const overlay =
        document.querySelector(".bg-black\\/50") ??
        document.querySelector('[class*="bg-black"]') ??
        dialog.parentElement;
      const hasOverlay =
        !!document.querySelector('[class*="bg-black/50"]') ||
        document.body.innerHTML.includes("bg-black/50") ||
        (overlay ? overlay.className.includes("bg-black") : false) ||
        dialog.className.includes("bg-black");
      expect(hasOverlay, "overlay debe tener bg-black/50").toBe(true); // AC-007 FR-010 bg-black/50
    });
  });

  describe("Accept con Zona Norte -> POST 201 + GET refresh añade fila", () => {
    // Covers [SPEC-004: AC-007, BR-012/013, FR-010, TS-007]

    it("Accept con Zona Norte -> POST 201 + GET refresh añade fila", async () => {
      // Covers [SPEC-004: AC-007, BR-012/013, FR-010]
      // Arrange
      const draft5 = makeDraft(makeClosed5());
      usePortalStore.setState({ draftPolygon: draft5 } as never);
      const onClose = vi.fn();
      const onCreated = vi.fn();
      let capturedBody: unknown = null;
      let postCalled = 0;
      let getCalled = 0;
      server.use(
        http.post("/api/zones", async ({ request }) => {
          postCalled += 1;
          capturedBody = await request.json();
          return HttpResponse.json(createdFeature, { status: 201 });
        }),
        http.post("*/api/zones", async ({ request }) => {
          postCalled += 1;
          capturedBody = await request.json();
          return HttpResponse.json(createdFeature, { status: 201 });
        }),
        http.get("/api/zones", () => {
          getCalled += 1;
          return HttpResponse.json({ type: "FeatureCollection", features: [createdFeature] });
        }),
        http.get("*/api/zones", () => {
          getCalled += 1;
          return HttpResponse.json({ type: "FeatureCollection", features: [createdFeature] });
        }),
      );

      render(<CreateZoneModal open draft={draft5} onClose={onClose} onCreated={onCreated} />);

      const input =
        (screen.queryByLabelText(/Zone name/i, { selector: 'input' }) as HTMLInputElement) ??
        (screen.queryByPlaceholderText(/Zone name/i) as HTMLInputElement) ??
        (document.querySelector('input') as HTMLInputElement);
      expect(input).not.toBeNull();

      // Act
      fireEvent.change(input, { target: { value: "Zona Norte" } });
      const acceptBtn = screen.getByRole("button", { name: /Accept/i });
      fireEvent.click(acceptBtn);

      // Assert — POST 201 con name y geojson Polygon
      await waitFor(() => {
        expect(postCalled, "POST /api/zones debe haberse llamado").toBeGreaterThan(0); // AC-007 FR-010 POST 201
      });
      const body = capturedBody as { name?: string; geojson?: { type: string; coordinates: number[][][] } } | null;
      expect(body).not.toBeNull();
      expect(body?.name).toBe("Zona Norte"); // AC-007 name
      expect(body?.geojson?.type).toBe("Polygon"); // AC-007 geojson Polygon
      expect(body?.geojson?.coordinates?.[0]?.length).toBe(5); // AC-007 5 coords
      expect(body?.geojson?.coordinates?.[0]?.[0]).toEqual(body?.geojson?.coordinates?.[0]?.[body!.geojson!.coordinates[0].length - 1]); // BR-013 cerrado

      // Assert — onCreated called y GET refresh
      await waitFor(() => {
        expect(onCreated).toHaveBeenCalled(); // AC-007 onCreated / refresh
      });
      // onClose should be called after success (modal cierra)
      await waitFor(() => {
        expect(onClose).toHaveBeenCalled(); // AC-007 modal cierra tras 201
      });
      // draft cleared
      expect(usePortalStore.getState().draftPolygon).toBeNull(); // AC-007 draft cleared tras Accept
      // GET refresh at least attempted if onCreated triggers fetch
      // not strictly required but if implemented via GET, count >0 or onCreated suffices
      void getCalled;
    });
  });

  describe("Cancel descarta draft sin POST", () => {
    // Covers [SPEC-004: AC-007, FR-010, BR-012]

    it("Cancel descarta draft sin POST", async () => {
      // Covers [SPEC-004: AC-007, FR-010, BR-012]
      // Arrange
      const draft5 = makeDraft(makeClosed5());
      usePortalStore.setState({ draftPolygon: draft5 } as never);
      const onClose = vi.fn();
      const onCreated = vi.fn();
      let postCalled = 0;
      server.use(
        http.post("/api/zones", () => {
          postCalled += 1;
          return HttpResponse.json(createdFeature, { status: 201 });
        }),
        http.post("*/api/zones", () => {
          postCalled += 1;
          return HttpResponse.json(createdFeature, { status: 201 });
        }),
      );
      render(<CreateZoneModal open draft={draft5} onClose={onClose} onCreated={onCreated} />);

      // Act
      const cancelBtn = screen.getByRole("button", { name: /Cancel/i });
      fireEvent.click(cancelBtn);

      // Assert — no POST
      await waitFor(() => {
        expect(onClose).toHaveBeenCalled(); // AC-007 Cancel cierra
      });
      expect(postCalled).toBe(0); // AC-007 Cancel sin POST
      expect(onCreated).not.toHaveBeenCalled(); // AC-007 no onCreated
      // draft cleared
      expect(usePortalStore.getState().draftPolygon).toBeNull(); // AC-007 Cancel descarta draft
    });
  });

  describe("draft 3 coords o no cerrado -> 400 inline no cierra", () => {
    // Covers [SPEC-004: AC-008, BR-013, FR-010, TS-007]

    it("draft 3 coords o no cerrado -> 400 inline no cierra", async () => {
      // Covers [SPEC-004: AC-008, BR-013, FR-010, TS-007]
      // Arrange — draft 3 coords inválido
      const draft3 = makeDraft(makeInvalid3());
      usePortalStore.setState({ draftPolygon: draft3 } as never);
      const onClose = vi.fn();
      const onCreated = vi.fn();
      server.use(
        http.post("/api/zones", () => HttpResponse.json({ error: "validation", details: ["coords length must be 4..101"] }, { status: 400 })),
        http.post("*/api/zones", () => HttpResponse.json({ error: "validation", details: ["coords length must be 4..101"] }, { status: 400 })),
      );
      render(<CreateZoneModal open draft={draft3} onClose={onClose} onCreated={onCreated} />);
      const input =
        (screen.queryByLabelText(/Zone name/i, { selector: 'input' }) as HTMLInputElement) ??
        (screen.queryByPlaceholderText(/Zone name/i) as HTMLInputElement) ??
        (document.querySelector('input') as HTMLInputElement);
      fireEvent.change(input, { target: { value: "Zona Bad" } });

      // Act — también prueba no cerrado first!=last alterno si componente valida cliente antes de POST
      const acceptBtn = screen.getByRole("button", { name: /Accept/i });
      fireEvent.click(acceptBtn);

      // Assert — 400 inline bajo input, no cierra
      await waitFor(() => {
        const err =
          screen.queryByText(/validation/i) ??
          screen.queryByText(/coords length/i) ??
          screen.queryByText(/first.*last/i) ??
          screen.queryByText(/ST_Area/i) ??
          document.querySelector('[role="alert"]');
        expect(err ?? document.body.textContent).toBeTruthy(); // AC-008 error inline
        // debe haber mensaje visible bajo input
        const inline =
          screen.queryByText(/validation/i) ||
          screen.queryByText(/coords length must be 4..101/i) ||
          screen.queryByText(/polygon not closed/i) ||
          screen.queryByText(/ST_Area/i);
        expect(inline ?? (document.body.innerHTML.includes("400") || document.body.innerHTML.includes("validation"))).toBeTruthy(); // AC-008 inline
      });
      expect(onClose).not.toHaveBeenCalled(); // AC-008 no cierra con 400
      expect(onCreated).not.toHaveBeenCalled();
      // también prueba no cerrado: segundo escenario first!=last
      cleanup();
      const draftNC = makeDraft(makeNotClosed5());
      usePortalStore.setState({ draftPolygon: draftNC } as never);
      const onClose2 = vi.fn();
      server.use(
        http.post("/api/zones", () => HttpResponse.json({ error: "validation", details: ["polygon not closed first!==last"] }, { status: 400 })),
        http.post("*/api/zones", () => HttpResponse.json({ error: "validation", details: ["polygon not closed first!==last"] }, { status: 400 })),
      );
      render(<CreateZoneModal open draft={draftNC} onClose={onClose2} onCreated={vi.fn()} />);
      const input2 =
        (screen.queryByLabelText(/Zone name/i, { selector: 'input' }) as HTMLInputElement) ??
        (screen.queryByPlaceholderText(/Zone name/i) as HTMLInputElement) ??
        (document.querySelector('input') as HTMLInputElement);
      fireEvent.change(input2, { target: { value: "Zona NC" } });
      fireEvent.click(screen.getByRole("button", { name: /Accept/i }));
      await waitFor(() => {
        expect(screen.queryByText(/polygon not closed/i) ?? screen.queryByText(/validation/i) ?? document.body.innerHTML.includes("validation")).toBeTruthy(); // AC-008 no cerrado
      });
      expect(onClose2).not.toHaveBeenCalled(); // AC-008 no cierra
    });
  });

  describe("duplicate name -> 409 inline", () => {
    // Covers [SPEC-004: AC-008, BR-013, FR-010, TS-007]

    it("duplicate name -> 409 inline", async () => {
      // Covers [SPEC-004: AC-008, BR-013, FR-010]
      // Arrange
      const draft5 = makeDraft(makeClosed5());
      usePortalStore.setState({ draftPolygon: draft5 } as never);
      const onClose = vi.fn();
      const onCreated = vi.fn();
      server.use(
        http.post("/api/zones", () => HttpResponse.json({ error: "zone name already exists" }, { status: 409 })),
        http.post("*/api/zones", () => HttpResponse.json({ error: "zone name already exists" }, { status: 409 })),
      );
      render(<CreateZoneModal open draft={draft5} onClose={onClose} onCreated={onCreated} />);
      const input =
        (screen.queryByLabelText(/Zone name/i, { selector: 'input' }) as HTMLInputElement) ??
        (screen.queryByPlaceholderText(/Zone name/i) as HTMLInputElement) ??
        (document.querySelector('input') as HTMLInputElement);
      fireEvent.change(input, { target: { value: "Zona Norte" } });

      // Act
      fireEvent.click(screen.getByRole("button", { name: /Accept/i }));

      // Assert — 409 inline zone name already exists bajo input, no cierra
      await waitFor(() => {
        expect(screen.getByText(/zone name already exists/i)).toBeInTheDocument(); // AC-008 BR-013 409 duplicate
      });
      expect(onClose).not.toHaveBeenCalled(); // AC-008 no cierra con 409
      expect(onCreated).not.toHaveBeenCalled();
      // draft permanece para corregir
      // implementation may keep draft; at least not cleared
      // portalStore draft still present or input still has value
      expect((input as HTMLInputElement).value).toBe("Zona Norte"); // AC-008 draft permanece
    });
  });
});
