/**
 * A row that can be held and dragged up or down its list. The drag runs on
 * the UI thread: the finger moves a shared value, and only the drop comes
 * back to React, with how far the row travelled. Move buttons next to the
 * rows do the same for keyboards and screen readers.
 */
import { motion } from "@nexus-chat/ui";
import { type ReactNode, useRef } from "react";
import type { LayoutChangeEvent } from "react-native";
import { GestureDetector, usePanGesture } from "react-native-gesture-handler";
import Animated, {
  type SharedValue,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { scheduleOnRN } from "react-native-worklets";

import { dropIndex, type RowLayout } from "../servers/dropOrder";

/** Props of {@link DraggableRow}. */
export interface DraggableRowProps {
  children: ReactNode;
  /** Whether the row can be dragged now; it stays put while a change is saved. */
  enabled: boolean;
  onLayout: (layout: RowLayout) => void;
  /** Called once when the row is dropped, with how far it was dragged. */
  onDrop: (translationY: number) => void;
  testID: string;
}

const { dragLiftScale, dragSettle, reducedMotionDragOpacity } = motion;

/** The lifted row's look: it follows the finger and sits above its neighbors. */
function useLiftedStyle(offset: SharedValue<number>, lifted: SharedValue<number>) {
  const reduced = motion.usePrefersReducedMotion();
  return useAnimatedStyle(() => {
    const up = lifted.get();
    return {
      zIndex: up,
      opacity: reduced && up === 1 ? reducedMotionDragOpacity : 1,
      transform: [{ translateY: offset.get() }, { scale: !reduced && up === 1 ? dragLiftScale : 1 }],
    };
  });
}

/**
 * Reports the row's layout, and when the row moves to another place in the
 * list (dropped there, or a neighbor was), slides it from where it was
 * shown instead of jumping. With reduced motion it moves at once.
 */
function useSlideOnMove(offset: SharedValue<number>, onLayout: (layout: RowLayout) => void) {
  const reduced = motion.usePrefersReducedMotion();
  const shownAtRef = useRef<number | undefined>(undefined);
  return (event: LayoutChangeEvent) => {
    const { y, height } = event.nativeEvent.layout;
    const before = shownAtRef.current;
    shownAtRef.current = y;
    if (before !== undefined && before !== y) {
      offset.set(reduced ? 0 : offset.get() + before - y);
      if (!reduced) {
        offset.set(withSpring(0, dragSettle));
      }
    }
    onLayout({ y, height });
  };
}

/** The pan gesture: held, it lifts the row; moved, the row follows; let go, it is dropped. */
function useDrag(props: DraggableRowProps, offset: SharedValue<number>, lifted: SharedValue<number>) {
  const { enabled, onDrop, testID } = props;
  return usePanGesture({
    testID,
    enabled,
    activateAfterLongPress: motion.dragHoldDelay,
    activeCursor: "grabbing",
    onActivate: () => {
      "worklet";
      lifted.set(1);
    },
    onUpdate: (event) => {
      "worklet";
      offset.set(event.translationY);
    },
    onDeactivate: (event) => {
      "worklet";
      if (!event.canceled) {
        scheduleOnRN(onDrop, event.translationY);
      }
    },
    onFinalize: () => {
      "worklet";
      lifted.set(0);
      offset.set(withSpring(0, dragSettle));
    },
  });
}

/** One draggable row of a reorderable list. */
export function DraggableRow(props: DraggableRowProps): ReactNode {
  const offset = useSharedValue(0);
  const lifted = useSharedValue(0);
  const style = useLiftedStyle(offset, lifted);
  const gesture = useDrag(props, offset, lifted);
  const layoutChanged = useSlideOnMove(offset, props.onLayout);
  return (
    <GestureDetector gesture={gesture}>
      <Animated.View style={style} onLayout={layoutChanged} testID={`${props.testID}-row`}>
        {props.children}
      </Animated.View>
    </GestureDetector>
  );
}

/** The layouts of a list's rows, for working out where a dragged row lands. */
export interface RowLayouts {
  /** Records where the row at `index` is. */
  track: (index: number) => (layout: RowLayout) => void;
  /** Where the row at `from`, dragged by `translationY`, lands among the others. */
  target: (from: number, translationY: number) => number;
}

/** Keeps the layouts of a list's draggable rows. */
export function useRowLayouts(): RowLayouts {
  const layoutsRef = useRef<RowLayout[]>([]);
  return {
    track: (index) => (layout) => {
      layoutsRef.current[index] = layout;
    },
    target: (from, translationY) => dropIndex(layoutsRef.current, from, translationY),
  };
}
