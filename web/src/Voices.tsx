import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Avatar,
  Button,
  Group,
  Paper,
  Select,
  Slider,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import {
  IconAlertCircle,
  IconCircleCheck,
  IconMicrophone,
  IconPlus,
  IconTrash,
  IconUsers,
} from "@tabler/icons-react";
import { api, fired, type Enrolment, type Person, type VoiceInfo, type VoiceLimits } from "./api";

const enrolmentEvent = "esphome.echolocal_enrolment";
const someone = "";

type Draft = { voice: string; person: string; name: string };

function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join("");
}

function status(stage: string): string {
  switch (stage) {
    case "heard":
      return "Got it";
    case "missed":
      return "Didn't catch that. Once more.";
    case "prompt":
      return "Listening…";
  }
  return "";
}

function Dots({ step }: { step: Enrolment }) {
  const of = Number(step.of);
  const total = step.confirm === "true" ? of : of + 1;
  const done = Number(step.take) - 1 + (step.stage === "heard" ? 1 : 0);
  return (
    <Group gap={10} justify="center">
      {Array.from({ length: total }, (_, i) => (
        <span
          key={i}
          style={{
            width: 10,
            height: 10,
            borderRadius: "50%",
            background: i < done ? "var(--mantine-primary-color-filled)" : i === done ? "transparent" : "var(--mantine-color-default-border)",
            boxShadow: i === done ? "inset 0 0 0 3px var(--mantine-primary-color-filled)" : undefined,
          }}
        />
      ))}
    </Group>
  );
}

type RowProps = {
  voice: VoiceInfo;
  limits: VoiceLimits;
  busy: boolean;
  onRecord: () => void;
  onThreshold: (t: number) => void;
  onRemove: () => void;
};

function VoiceRow({ voice, limits, busy, onRecord, onThreshold, onRemove }: RowProps) {
  const [threshold, setThreshold] = useState(voice.threshold);
  const [removing, setRemoving] = useState(false);
  useEffect(() => setThreshold(voice.threshold), [voice.threshold]);
  return (
    <Paper withBorder p="sm">
      <Stack gap="xs">
        <Group justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap" style={{ minWidth: 0 }}>
            <Avatar radius="xl" color="initials" name={voice.name}>
              {initials(voice.name)}
            </Avatar>
            <Text fw={500} truncate>
              {voice.name}
            </Text>
          </Group>
          {removing ? (
            <Group gap="xs" wrap="nowrap">
              <Button size="xs" variant="subtle" onClick={() => setRemoving(false)}>
                Cancel
              </Button>
              <Button size="xs" color="red" onClick={onRemove}>
                Remove
              </Button>
            </Group>
          ) : (
            <Group gap="xs" wrap="nowrap">
              <Button
                size="xs"
                variant="subtle"
                leftSection={<IconMicrophone size={14} />}
                disabled={busy}
                onClick={onRecord}
              >
                Record again
              </Button>
              <ActionIcon
                variant="subtle"
                color="gray"
                aria-label="Remove"
                disabled={busy}
                onClick={() => setRemoving(true)}
              >
                <IconTrash size={18} />
              </ActionIcon>
            </Group>
          )}
        </Group>
        <Group gap="sm" wrap="nowrap">
          <Text size="xs" c="dimmed" w={110}>
            Match threshold
          </Text>
          <Slider
            style={{ flex: 1 }}
            min={limits.loosest}
            max={limits.strictest}
            step={0.01}
            value={threshold}
            onChange={setThreshold}
            onChangeEnd={onThreshold}
            label={null}
          />
          <Text size="sm" ff="monospace" w={40} ta="right">
            {threshold.toFixed(2)}
          </Text>
        </Group>
      </Stack>
    </Paper>
  );
}

export function Voices({ onSignedOut }: { onSignedOut: (e: unknown) => void }) {
  const [limits, setLimits] = useState<VoiceLimits>({ loosest: 0.3, strictest: 0.7, default: 0.5 });
  const [voices, setVoices] = useState<VoiceInfo[]>([]);
  const [people, setPeople] = useState<Person[]>([]);
  const [peopleError, setPeopleError] = useState("");
  const [step, setStep] = useState<Enrolment>({ stage: "idle" });
  const [draft, setDraft] = useState<Draft>();
  const [last, setLast] = useState<Draft>();
  const [error, setError] = useState("");

  const load = useCallback(() => {
    api
      .voices()
      .then((p) => {
        setLimits(p.limits);
        setVoices(p.voices);
        setPeople(p.people);
        setPeopleError(p.people_error ?? "");
        setStep(p.enrolment);
      })
      .catch(onSignedOut);
  }, [onSignedOut]);

  useEffect(load, [load]);

  useEffect(
    () =>
      fired(enrolmentEvent, (data) => {
        setStep(data);
        if (data.stage === "ready") load();
      }),
    [load],
  );

  const nameOf = (person: string) => people.find((p) => p.id === person)?.name ?? "";

  const add = (voice?: VoiceInfo) => {
    const taken = new Set(voices.map((v) => v.person).filter(Boolean));
    const person = voice ? voice.person : (people.find((p) => !taken.has(p.id))?.id ?? someone);
    setError("");
    setDraft({ voice: voice?.id ?? "", person, name: voice?.name ?? nameOf(person) });
  };

  const pick = (value: string | null) => {
    if (!draft) return;
    const person = value ?? someone;
    const named = draft.name.trim() === "" || draft.name === nameOf(draft.person);
    setDraft({ ...draft, person, name: named ? nameOf(person) : draft.name });
  };

  const start = async (d: Draft) => {
    setError("");
    try {
      setStep(await api.enroll(d.name.trim(), d.person, d.voice));
      setLast(d);
      setDraft(undefined);
    } catch (e) {
      onSignedOut(e);
      setError(e instanceof Error ? e.message : "Try again");
    }
  };

  const run = async (work: Promise<void>) => {
    setError("");
    try {
      await work;
    } catch (e) {
      onSignedOut(e);
      setError(e instanceof Error ? e.message : "Try again");
    }
    load();
  };

  const stop = () => run(api.cancelEnrolment());

  const running = step.stage !== "idle";
  const lead = step.wake ? `Say “${step.wake}”, then` : "Say the wake word, then";

  return (
    <Stack gap="md" maw={720}>
      <Group justify="space-between">
        <Title order={3}>Voice match</Title>
        {!running && !draft && (
          <Button leftSection={<IconPlus size={16} />} variant="light" onClick={() => add()}>
            Add voice
          </Button>
        )}
      </Group>

      {peopleError && (
        <Alert color="yellow">Home Assistant's people aren't available: {peopleError}</Alert>
      )}

      {running && (
        <Paper withBorder p="xl">
          <Stack gap="md" align="center" ta="center">
            {step.stage === "ready" ? (
              <>
                <Group gap="xs">
                  <IconCircleCheck size={28} color="var(--mantine-color-green-filled)" />
                  <Text fw={600} size="lg">
                    Voice match is ready for {step.name}
                  </Text>
                </Group>
                <Button onClick={() => void stop()}>Done</Button>
              </>
            ) : step.stage === "unconfirmed" || step.stage === "failed" ? (
              <>
                <Group gap="xs">
                  <IconAlertCircle size={28} color="var(--mantine-color-red-filled)" />
                  <Text fw={600} size="lg">
                    {step.stage === "unconfirmed" ? "That didn't sound like the same voice" : "Voice match couldn't be set up"}
                  </Text>
                </Group>
                <Group>
                  <Button variant="subtle" onClick={() => void stop()}>
                    Cancel
                  </Button>
                  {last && <Button onClick={() => void start(last)}>Try again</Button>}
                </Group>
              </>
            ) : step.stage === "similar" ? (
              <>
                <Group gap="xs">
                  <IconUsers size={28} color="var(--mantine-color-yellow-filled)" />
                  <Text fw={600} size="lg">
                    This voice sounds very similar to {step.like}
                  </Text>
                </Group>
                <Group>
                  <Button variant="subtle" onClick={() => void stop()}>
                    Cancel
                  </Button>
                  {last && <Button onClick={() => void start(last)}>Try again</Button>}
                  <Button variant="default" onClick={() => void run(api.acceptEnrolment())}>
                    Accept
                  </Button>
                </Group>
              </>
            ) : step.phrase ? (
              <>
                <Text c="dimmed">
                  {lead}
                </Text>
                <Text fw={600} size="xl" c="var(--mantine-primary-color-filled)" style={{ textWrap: "balance" }}>
                  “{step.phrase}”
                </Text>
                <Dots step={step} />
                <Text mih="1.4em">{status(step.stage)}</Text>
                <Button variant="subtle" onClick={() => void stop()}>
                  Cancel
                </Button>
              </>
            ) : (
              <>
                <Text c="dimmed">Getting ready…</Text>
                <Button variant="subtle" onClick={() => void stop()}>
                  Cancel
                </Button>
              </>
            )}
          </Stack>
        </Paper>
      )}

      {draft && !running && (
        <Paper withBorder p="md">
          <Stack gap="sm">
            <Select
              label="Person"
              value={draft.person}
              allowDeselect={false}
              data={[...people.map((p) => ({ value: p.id, label: p.name })), { value: someone, label: "Someone else" }]}
              onChange={pick}
            />
            <TextInput
              label="Name"
              value={draft.name}
              onChange={(e) => setDraft({ ...draft, name: e.currentTarget.value })}
            />
            <Group justify="flex-end">
              <Button variant="subtle" onClick={() => setDraft(undefined)}>
                Cancel
              </Button>
              <Button disabled={!draft.name.trim()} onClick={() => void start(draft)}>
                Start
              </Button>
            </Group>
          </Stack>
        </Paper>
      )}

      {error && (
        <Text size="sm" c="red">
          {error}
        </Text>
      )}

      {voices.length === 0 ? (
        <Text c="dimmed">No voices yet.</Text>
      ) : (
        <Stack gap="xs">
          {voices.map((v) => (
            <VoiceRow
              key={v.id}
              voice={v}
              limits={limits}
              busy={running || !!draft}
              onRecord={() => add(v)}
              onThreshold={(t) => void run(api.setThreshold(v.id, t))}
              onRemove={() => void run(api.removeVoice(v.id))}
            />
          ))}
        </Stack>
      )}
    </Stack>
  );
}
