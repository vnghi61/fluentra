import { describe, expect, it } from "vitest";

import { reachableStorageUrl } from "@/lib/storage-url";

const signed =
  "http://localhost:9000/fluentra-uploads/recordings/u/a.webm?X-Amz-Signature=abc";

describe("reachableStorageUrl", () => {
  it("leaves a development store alone on the machine running it", () => {
    const page = { hostname: "localhost", origin: "http://localhost:5173" };
    expect(reachableStorageUrl(signed, page)).toBe(signed);
  });

  it("sends a phone on the LAN through the dev server, signature intact", () => {
    // `localhost` on the phone is the phone: the upload never left it.
    const page = {
      hostname: "192.168.1.20",
      origin: "https://192.168.1.20:5173",
    };
    expect(reachableStorageUrl(signed, page)).toBe(
      "https://192.168.1.20:5173/__storage/fluentra-uploads/recordings/u/a.webm?X-Amz-Signature=abc",
    );
  });

  it("maps the compose host name to localhost on the same machine", () => {
    const page = { hostname: "localhost", origin: "http://localhost:5173" };
    expect(
      reachableStorageUrl("http://minio:9000/bucket/key?sig=1", page),
    ).toBe("http://localhost:9000/bucket/key?sig=1");
  });

  it("passes a production store through untouched", () => {
    const url = "https://media.example.com/tts/a.wav?X-Amz-Expires=105";
    const page = {
      hostname: "app.example.com",
      origin: "https://app.example.com",
    };
    expect(reachableStorageUrl(url, page)).toBe(url);
  });
});
